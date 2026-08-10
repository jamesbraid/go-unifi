#!/usr/bin/env bash
set -euo pipefail

# Skunkworks-only live path. The in-repo provisioner is a separate process: it
# inspects the started container, locked image, and controller API and emits the
# Task 2 receipt consumed independently by scout and campaign validation.
#
# The target is read from the scout profile rather than restated here. The
# profile already names the version and both image digests, and scout is already
# handed it below, so hardcoding the same three facts gave them a second home
# that nothing compared against the first. It also pinned this script to
# 10.4.57, which meant the one scenario the live path exists for -- a new
# controller version -- was refused before anything ran.
readonly builder_image=golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651
readonly workflow=.woodpecker/m4-compatibility-campaigns.yml
readonly capture_lock=${CAMPAIGN_CAPTURE_LOCK:-schemas/capture.lock.json}
if [[ ! -r ${capture_lock} ]]; then
    echo "capture lock not readable: ${capture_lock}" >&2
    exit 1
fi
lock_version=$(jq -er '.controller.network_version' "${capture_lock}")
readonly lock_version

# The profile follows the lock instead of naming a version here. A literal
# default meant the capture pipeline could advance the lock to a new controller
# while the campaign still reached for the old version's profile, and the two
# then disagreed by construction -- the cross-check below would fire on every
# upgrade, reporting a mismatch this script had caused itself.
readonly target_profile=${CAMPAIGN_TARGET_PROFILE:-scout/profiles/network-${lock_version}-seeded.json}
if [[ ! -r ${target_profile} ]]; then
    echo "target profile not readable: ${target_profile}" >&2
    echo "  the capture lock names Network ${lock_version}, so that is the profile required" >&2
    echo "  a controller version with no profile has not been onboarded; produce one with" >&2
    echo "  .woodpecker/scripts/onboard-controller-profile.sh ${lock_version}" >&2
    echo "  or set CAMPAIGN_TARGET_PROFILE to the profile naming the controller under test" >&2
    exit 1
fi

locked_version=$(jq -er '.version' "${target_profile}")
locked_repository=$(jq -er '.image_repository' "${target_profile}")
locked_index=$(jq -er '.image_index_sha256' "${target_profile}")
locked_manifest=$(jq -er '.image_manifest_sha256' "${target_profile}")
readonly locked_version locked_repository locked_index locked_manifest
readonly locked_reference="${locked_repository}@${locked_index}"

# The profile says which controller is being exercised; the capture lock says
# which controller the committed schema was extracted from. Comparing them
# catches the run that would otherwise succeed while proving nothing: a live
# controller measured against a schema generated from a different version.
if [[ ${locked_version} != "${lock_version}" ]]; then
    echo "target profile and capture lock disagree about the controller version" >&2
    echo "  profile      ${target_profile}: ${locked_version}" >&2
    echo "  capture lock ${capture_lock}: ${lock_version}" >&2
    echo "  regenerate against this controller, or point CAMPAIGN_TARGET_PROFILE at the matching profile" >&2
    exit 1
fi
# The baseline is deliberately NOT derived from the target profile. It is the
# previously admitted catalog, which for a new controller is the OLD version's
# -- comparing a 10.5 candidate against a 10.5 baseline would compare a thing to
# itself. Today 10.4.57 is the only admitted version, so this default is both
# current and the correct predecessor. It becomes wrong the moment a second
# version is admitted, which is why it is an override rather than a literal.
readonly baseline_catalog=${CAMPAIGN_BASELINE_CATALOG:-catalogs/network-10.4.57/dns_record.admitted-catalog.json}
readonly baseline_admission_receipt=${CAMPAIGN_BASELINE_ADMISSION_RECEIPT:-catalogs/network-10.4.57/dns_record.admission-receipt.json}
if [[ ! -r ${baseline_catalog} ]]; then
    echo "baseline catalog not readable: ${baseline_catalog}" >&2
    echo "  set CAMPAIGN_BASELINE_CATALOG to the previously admitted catalog" >&2
    exit 1
fi

work_root=${CAMPAIGN_LIVE_OUTPUT_DIR:-"$(pwd)/.campaign-results/${CI_PIPELINE_NUMBER:-manual}"}
readonly work_root
readonly container_name="go-unifi-dns-campaign-${CI_PIPELINE_NUMBER:-manual}"
readonly attempt_id=${CAMPAIGN_ATTEMPT_ID:-"live-attempt-${CI_PIPELINE_NUMBER:-manual}"}
readonly outer_ledger="${work_root}/outer-attempts.ndjson"
container_id=""
cleanup() {
    local result=$?
    trap - EXIT
    if [[ -n "${container_id}" ]]; then
        docker stop "${container_id}" >/dev/null || true
    fi
    local state=succeeded
    if [[ "${result}" -ne 0 ]]; then
        state=failed
    fi
    jq -cn \
        --arg attempt_id "${attempt_id}" \
        --arg state "${state}" \
        --argjson exit_code "${result}" \
        --arg finished_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        '{format_version:1,attempt_id:$attempt_id,state:$state,stage:"live_campaign",exit_code:$exit_code,finished_at:$finished_at}' \
        >>"${outer_ledger}" || true
    sync || true
    jq -c . "${outer_ledger}" || true
    echo "retained live evidence: ${work_root}"
    exit "${result}"
}
trap cleanup EXIT
mkdir -p "${work_root}"
jq -cn \
    --arg attempt_id "${attempt_id}" \
    --arg pipeline_identity "${CI_REPO:-local}#${CI_PIPELINE_NUMBER:-manual}" \
    --arg started_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '{format_version:1,attempt_id:$attempt_id,state:"started",stage:"provisioning",pipeline_identity:$pipeline_identity,started_at:$started_at}' \
    >>"${outer_ledger}"
sync

# Checked explicitly rather than with ${VAR:?message}. Measured: when the shell
# dies from a :? expansion error, an EXIT trap observes $? as 0 and the script
# exits 0 -- so every one of these checks reported success while doing nothing.
# The trap form does not matter; any EXIT trap does it. Explicit exit 1 works.
if [[ -z ${HOSTNAME:-} ]]; then
    echo "HOSTNAME must identify the Woodpecker step container" >&2
    exit 1
fi
if [[ -z ${UNIFI_USERNAME:-} ]]; then
    echo "UNIFI_USERNAME is required" >&2
    exit 1
fi
if [[ -z ${UNIFI_PASSWORD:-} ]]; then
    echo "UNIFI_PASSWORD is required" >&2
    exit 1
fi

# The profile is the source of the reference, so an unset UNIFI_NETWORK_IMAGE
# is not an error -- it is the ordinary case. Requiring it made a fact the
# profile already carries live in a second place that a human has to update on
# every controller upgrade, and the campaign refused the upgrade until they
# did, for a reason that had nothing to do with the upgrade.
#
# When it IS set it is still checked, because then it is an independent claim
# about which image the pool will run and disagreement is worth catching. The
# whole reference, not just the digest suffix: a digest-only check passes for
# the right digest served from the wrong registry.
if [[ -z ${UNIFI_NETWORK_IMAGE:-} ]]; then
    UNIFI_NETWORK_IMAGE=${locked_reference}
    echo "image not supplied; using the one ${target_profile} locks"
    echo "  ${UNIFI_NETWORK_IMAGE}"
elif [[ ${UNIFI_NETWORK_IMAGE} != "${locked_reference}" ]]; then
    echo "UNIFI_NETWORK_IMAGE is not the image this target profile locks" >&2
    echo "  profile:  ${target_profile} (${locked_version})" >&2
    echo "  expected: ${locked_reference}" >&2
    echo "  got:      ${UNIFI_NETWORK_IMAGE}" >&2
    echo "  the profile is the single source for the reference; update it, not the secret" >&2
    exit 1
fi

docker_args=(--detach --rm --name "${container_name}" --network "container:${HOSTNAME}")
if [[ -n "${CAMPAIGN_NETWORK_ENV_FILE:-}" ]]; then
    docker_args+=(--env-file "${CAMPAIGN_NETWORK_ENV_FILE}")
fi
container_id=$(docker run "${docker_args[@]}" "${UNIFI_NETWORK_IMAGE}")
readonly container_id

readonly provisioner_receipt="${work_root}/provisioner-receipt.json"
export UNIFI_API="${UNIFI_API:-https://127.0.0.1:8443}"
export UNIFI_INSECURE="${UNIFI_INSECURE:-true}"
go run ./cmd/provisioner-receipt \
    --container-id "${container_id}" \
    --image "${UNIFI_NETWORK_IMAGE}" \
    --expected-version "${locked_version}" \
    --expected-image-index "${locked_index}" \
    --expected-image-manifest "${locked_manifest}" \
    --output "${provisioner_receipt}"

go run ./cmd/campaign-receipt \
    -workflow "${workflow}" \
    -provisioner-receipt "${provisioner_receipt}" \
    -output "${work_root}/runner-execution-receipt.json"

go run ./cmd/scout \
    -target-profile "${target_profile}" \
    -target-receipt "${provisioner_receipt}" \
    -scenario scout/scenarios/dns-record-list-v1.json \
    -structural schemas/structural/dns_record.json \
    -semantic-predecessor schemas/semantic-ids/dns_record.previous.json \
    -semantic-ids schemas/semantic-ids/dns_record.json \
    -capture-lock "${capture_lock}" \
    -catalog-output "${work_root}/candidate.catalog.json" \
    -receipt-output "${work_root}/scenario.receipt.json"

go run ./cmd/campaign \
    -campaign-id "network-${locked_version}-dns-record-live" \
    -profile-class fresh_seeded \
    -builder-image "${builder_image}" \
    -baseline "${baseline_catalog}" \
    -admission-receipt "${baseline_admission_receipt}" \
    -candidate "${work_root}/candidate.catalog.json" \
    -receipt "${work_root}/scenario.receipt.json" \
    -execution-receipt "${work_root}/runner-execution-receipt.json" \
    -provisioner-receipt "${provisioner_receipt}" \
    -runner-workflow "${workflow}" \
    -attempt-ledger "${work_root}/attempts.ndjson" \
    -attempt-id "${attempt_id}" \
    -decision "candidate-only live compatibility evidence" \
    -output "${work_root}/attestation.json"

jq -e '.candidate_only == true and .promotion == "review_required" and .execution_mode == "runner"' "${work_root}/attestation.json" >/dev/null
shasum -a 256 "${provisioner_receipt}" "${work_root}/runner-execution-receipt.json" "${work_root}/scenario.receipt.json" "${work_root}/attestation.json"
jq -c . "${work_root}/attempts.ndjson"
