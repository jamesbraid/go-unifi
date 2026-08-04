#!/usr/bin/env bash
set -euo pipefail

# Skunkworks-only live path. The in-repo provisioner is a separate process: it
# inspects the started container, locked image, and controller API and emits the
# Task 2 receipt consumed independently by scout and campaign validation.
readonly locked_index=584be3a2e45c4913e1bc373eff9c7330609c82085d4fc6f5ea365abdcdb3e664
readonly locked_manifest=9d19c8d03948a77d28181743fb81a515aa51a5cea0856bc0491b34d92a01bcf4
readonly builder_image=golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651
readonly workflow=.woodpecker/m4-compatibility-campaigns.yml
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

: "${HOSTNAME:?HOSTNAME must identify the Woodpecker step container}"
: "${UNIFI_NETWORK_IMAGE:?UNIFI_NETWORK_IMAGE must name the locked Network image}"
: "${UNIFI_USERNAME:?UNIFI_USERNAME is required}"
: "${UNIFI_PASSWORD:?UNIFI_PASSWORD is required}"

if [[ "${UNIFI_NETWORK_IMAGE}" != *@sha256:${locked_index} ]]; then
    echo "UNIFI_NETWORK_IMAGE does not match the locked Network index" >&2
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
    --expected-version 10.4.57 \
    --expected-image-index sha256:584be3a2e45c4913e1bc373eff9c7330609c82085d4fc6f5ea365abdcdb3e664 \
    --expected-image-manifest sha256:${locked_manifest} \
    --output "${provisioner_receipt}"

go run ./cmd/campaign-receipt \
    -workflow "${workflow}" \
    -provisioner-receipt "${provisioner_receipt}" \
    -output "${work_root}/runner-execution-receipt.json"

go run ./cmd/scout \
    -target-profile scout/profiles/network-10.4.57-seeded.json \
    -target-receipt "${provisioner_receipt}" \
    -scenario scout/scenarios/dns-record-list-v1.json \
    -structural schemas/structural/dns_record.json \
    -semantic-predecessor schemas/semantic-ids/dns_record.previous.json \
    -semantic-ids schemas/semantic-ids/dns_record.json \
    -capture-lock schemas/capture.lock.json \
    -catalog-output "${work_root}/candidate.catalog.json" \
    -receipt-output "${work_root}/scenario.receipt.json"

go run ./cmd/campaign \
    -campaign-id network-10.4.57-dns-record-live \
    -profile-class fresh_seeded \
    -builder-image "${builder_image}" \
    -baseline catalogs/network-10.4.57/dns_record.admitted-catalog.json \
    -admission-receipt catalogs/network-10.4.57/dns_record.admission-receipt.json \
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
