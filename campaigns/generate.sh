#!/usr/bin/env bash
set -euo pipefail

readonly builder_image=golang:1.26.5-bookworm@sha256:1ecb7edf62a0408027bd5729dfd6b1b8766e578e8df93995b225dfd0944eb651
readonly target=network-10.4.57
readonly baseline=../catalogs/${target}/dns_record.admitted-catalog.json
readonly candidate=../catalogs/${target}/dns_record.catalog.json
readonly receipt=../catalogs/${target}/dns_record.scenario-receipt.json
work_root=$(mktemp -d "${TMPDIR:-/tmp}/go-unifi-campaign.XXXXXX")
readonly work_root
cleanup() {
    local result=$?
    trap - EXIT
    rm -rf "${work_root}"
    exit "${result}"
}
trap cleanup EXIT

claim_args=()
for field in enabled key port priority record_type ttl value weight; do
    claim_args+=("-reconfirmed-claim" "unifi.network.dns_record.field.${field}")
done

controller_fingerprint=$(jq -er '.target.controller_fingerprint' "${candidate}")
readonly controller_fingerprint
attempt_json=$(jq -cn \
    --arg builder_image "${builder_image}" \
    --arg controller_fingerprint "${controller_fingerprint}" \
    '{
      id: "fixture-attempt-1",
      outcome: "completed",
      execution_mode: "fixture",
      builder_image_digest: $builder_image,
      builder_provenance: "fixture",
      controller_fingerprint: $controller_fingerprint,
      controller_provenance: "fixture",
      runtime_provenance: "fixture"
    }')
readonly attempt_json

jq -e '.admission.state == "admitted"' "${baseline}" >/dev/null
if [[ "$(realpath "${baseline}")" == "$(realpath "${candidate}")" ]]; then
    echo "admitted baseline and candidate resolve to the same path" >&2
    exit 1
fi

go run ../cmd/campaign \
    -campaign-id network-10.4.57-dns-record \
    -profile-class fresh_seeded \
    -builder-image "${builder_image}" \
    -baseline "${baseline}" \
    -candidate "${candidate}" \
    -receipt "${receipt}" \
    -attempt-json "${attempt_json}" \
    -elapsed-milliseconds 0 \
    -decision "offline deterministic replay" \
    "${claim_args[@]}" \
    -output ${target}/dns_record.attestation.json

go run ../cmd/scout \
    -target-profile ../scout/profiles/network-10.4.57-seeded.json \
    -scenario ../scout/scenarios/dns-record-list-v1.json \
    -structural ../schemas/structural/dns_record.json \
    -semantic-predecessor ../schemas/semantic-ids/dns_record.previous.json \
    -semantic-ids ../schemas/semantic-ids/dns_record.json \
    -capture-lock ../schemas/capture.lock.json \
    -response ../scout/fixtures/dns-record-list-v1-vendor-mutation.json \
    -catalog-output "${work_root}/mutated.catalog.json" \
    -receipt-output "${work_root}/mutated.receipt.json"

go run ../cmd/campaign \
    -campaign-id network-10.4.57-dns-record-mutation \
    -profile-class fresh_seeded \
    -builder-image "${builder_image}" \
    -baseline "${baseline}" \
    -candidate "${work_root}/mutated.catalog.json" \
    -receipt "${work_root}/mutated.receipt.json" \
    -attempt-json "${attempt_json}" \
    -elapsed-milliseconds 0 \
    -decision "reject unreviewed vendor field" \
    "${claim_args[@]}" \
    -output ${target}/dns_record.mutation-attestation.json

jq -e '.classification == "unchanged" and .promotion == "review_required" and .candidate_only == true and (.inputs.baseline_catalog_sha256 != .inputs.candidate_catalog_sha256)' \
    ${target}/dns_record.attestation.json >/dev/null
jq -e '.classification == "capture_invalid" and .promotion == "review_required" and .candidate_only == true' \
    ${target}/dns_record.mutation-attestation.json >/dev/null
