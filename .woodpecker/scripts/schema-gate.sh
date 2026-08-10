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

# Said out loud on both paths. Only the rejection branch announced the
# classification, so an allowed run never recorded the verdict it acted on --
# and when a later step failed, the one fact everybody wanted was missing from
# the log despite having been read at the top of this script.
echo "gate: classification=${classification}"

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
