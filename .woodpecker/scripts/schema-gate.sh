#!/usr/bin/env bash
set -euo pipefail

# The gate. Binds the attestation to the commit it describes, decides whether a
# pull request may exist, publishes the evidence either way, and opens the PR
# only when the classification allows it.
#
# ALLOW-LIST, NOT DENY-LIST. The classifier assigns in a last-writer-wins
# cascade (internal/campaign/attestation.go): unchanged -> additive_candidate ->
# breaking_suspect -> capture_invalid -> generator_defect. Two consequences.
# Blocking on a named pair misses generator_defect, which means the generated
# tree does not agree with itself and is no safer to merge than a break. And
# only one classification survives the cascade, so the ABSENCE of a breaking
# change cannot be inferred from the value reported. Only an allow-list is
# sound.
#
# THE BINDING. Under this ordering the attestation exists before the pull
# request does, so nothing positional connects them. Without the head_sha
# assertion an attestation produced against one tip could be attached to a PR
# proposing another, and nothing would notice -- the same replay this apparatus
# binds controller identity to prevent, left open on the candidate side.
#
# Usage: schema-gate.sh <evidence-dir> <head-branch> <base-branch>

readonly evidence_dir=${1:?evidence directory is required}
readonly head=${2:?head branch is required}
readonly base=${3:?base branch is required}
readonly attestation="${evidence_dir}/attestation.json"
readonly capture_lock=${CAMPAIGN_CAPTURE_LOCK:-schemas/capture.lock.json}
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
readonly script_dir

for required in "${attestation}" "${capture_lock}"; do
    if [[ ! -r ${required} ]]; then
        echo "gate: required input not readable: ${required}" >&2
        exit 1
    fi
done

head_sha=${CI_COMMIT_SHA:?CI_COMMIT_SHA is required}
classification="$(jq -er '.classification' "${attestation}")"
readonly head_sha classification

# Announced here, before anything that can fail. Saying it next to the decision
# read better and was useless: publishing sits in between, and when publishing
# failed the run ended without ever stating the verdict it had already read one
# line earlier. A fact worth logging is logged where nothing can pre-empt it.
echo "gate: classification=${classification}"

# And why. The verdict alone cannot be acted on: capture_invalid on a known-good
# version is either the loop reporting a real gap in the capture or the harness
# producing an artefact of its own, and those imply opposite work. The reason
# lives in the attestation, which is uploaded rather than printed -- so when the
# upload failed, the run stated a verdict nobody could check.
#
# conflicts come from internal/scout/catalog.go: unknown_observed_field, where
# the live controller returned a field the structural projection does not
# declare, and type_mismatch, with both types named. A blocked admission with no
# conflicts means the coverage was incomplete instead.
# admission.state is NOT in the attestation -- it lives in the candidate catalog
# the campaign leaves beside it. Both halves of the condition that produced the
# verdict have to be visible, or a reader can only see one of the two things
# that could have caused it.
readonly candidate_catalog="${evidence_dir}/candidate.catalog.json"
if [[ -r ${candidate_catalog} ]]; then
    echo "gate: admission.state=$(jq -r '.admission.state // "absent"' "${candidate_catalog}")"
else
    echo "gate: admission.state unavailable (${candidate_catalog} not readable)"
fi

conflict_count="$(jq -r '(.conflicts // []) | length' "${attestation}")"
if [[ ${conflict_count} -gt 0 ]]; then
    echo "gate: ${conflict_count} conflict(s) behind that verdict:"
    jq -r '(.conflicts // [])[]
        | "  - \(.kind): \(.field)"
        + (if .expected != null and .expected != "" then " (declared \(.expected), observed \(.observed))" else "" end)' \
        "${attestation}"
else
    # No conflicts is the interesting case, not the empty one. Admission also
    # blocks when a field the structural projection DECLARES was never returned
    # -- the mirror of an undeclared field appearing, and it produces exactly
    # this shape: blocked, zero conflicts. Demonstrated by removing one field
    # from the fixture: admission_state=blocked, conflicts=0, and the field
    # named in coverage. So name them, or the reader is told there is no reason.
    unobserved="$(jq -r '[(.claim_coverage // [])[] | select(.state != "observed") | .id] | join(", ")' "${attestation}")"
    if [[ -n ${unobserved} ]]; then
        echo "gate: no conflicts, but the capture did not exercise every declared field:"
        echo "  not observed: ${unobserved}"
        echo "  a declared field the controller never returned blocks admission, so a"
        echo "  live target holding no data for it cannot produce an admitted capture"
    else
        echo "gate: no conflicts and full coverage recorded; any blocking verdict came from neither"
    fi
fi
promotion="$(jq -r '.promotion // "unstated"' "${attestation}")"
echo "gate: promotion=${promotion} candidate_only=$(jq -r '.candidate_only // "unstated"' "${attestation}")"

# The join receipt. Positional association is not association.
readonly receipt="${evidence_dir}/join-receipt.json"
jq -n \
    --arg head_sha "${head_sha}" \
    --arg capture_lock_sha256 "$(sha256sum "${capture_lock}" | cut -d' ' -f1)" \
    --arg attestation_sha256 "$(sha256sum "${attestation}" | cut -d' ' -f1)" \
    --arg classification "${classification}" \
    '{format_version:1,
      head_sha:$head_sha,
      capture_lock_sha256:$capture_lock_sha256,
      attestation_sha256:$attestation_sha256,
      classification:$classification}' >"${receipt}"
echo "gate: join receipt binds attestation to ${head_sha}"

# Evidence is published whatever the verdict. A rejected candidate is the case
# where someone most needs to read why.
"${script_dir}/publish-evidence.sh" "${evidence_dir}" "${head_sha}"

case "${classification}" in
    unchanged|additive_candidate)
        echo "gate: permitted for a pull request"
        ;;
    *)
        echo
        echo "gate: NO PULL REQUEST. classification=${classification}"
        echo "  Permitted for a pull request: unchanged, additive_candidate."
        echo "  The branch ${head} and its evidence are retained. A human decides."
        echo "  This is not a failure of the campaign. It is the campaign reporting"
        echo "  that the candidate is not mergeable on its own evidence."
        exit 1
        ;;
esac

body="$(mktemp)"
trap 'rm -f "${body}"' EXIT
{
    printf 'Schema refresh for `%s`.\n\n' "$(jq -r '.controller.build' "${capture_lock}")"
    printf 'Classification **%s**, from a campaign against a live controller.\n\n' "${classification}"
    printf '| | |\n|---|---|\n'
    printf '| head | `%s` |\n' "${head_sha}"
    printf '| controller | `%s` |\n' "$(jq -r '.controller.network_version' "${capture_lock}")"
    printf '| attestation | `%s` |\n' "$(jq -r '.attestation_sha256' "${receipt}")"
    printf '| capture lock | `%s` |\n\n' "$(jq -r '.capture_lock_sha256' "${receipt}")"
    if [[ -r ${evidence_dir}/apidiff-summary.md ]]; then
        cat "${evidence_dir}/apidiff-summary.md"
        printf '\n'
    fi
    printf 'Evidence, including the hash-chained attempt ledger, is published\n'
    printf 'alongside this branch rather than attached here, so that a rejected\n'
    printf 'candidate keeps its evidence too.\n'
} >"${body}"

"${script_dir}/open-schema-pr.sh" "${head}" "${base}" \
    "Update to $(jq -r '.controller.build' "${capture_lock}")" "${body}"
