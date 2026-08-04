#!/usr/bin/env bash
set -euo pipefail

readonly builder_image=local/go-unifi-schema-builder@sha256:0c4025799ff17a705b56690634e7c159f48ec500f4dedfd2d1bae9e8a016ba21
readonly target=network-10.4.57
readonly catalog=../catalogs/${target}/dns_record.catalog.json
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

go run ../cmd/campaign \
    -campaign-id network-10.4.57-dns-record \
    -profile-class fresh_seeded \
    -builder-image "${builder_image}" \
    -baseline "${catalog}" \
    -candidate "${catalog}" \
    -receipt "${receipt}" \
    -elapsed-milliseconds 0 \
    -decision "offline deterministic replay" \
    "${claim_args[@]}" \
    -output ${target}/dns_record.attestation.json

go run ../cmd/scout \
    -target-profile ../scout/profiles/network-10.4.57-seeded.json \
    -scenario ../scout/scenarios/dns-record-list-v1.json \
    -structural ../schemas/structural/dns_record.json \
    -semantic-ids ../schemas/semantic-ids/dns_record.json \
    -capture-lock ../schemas/capture.lock.json \
    -response ../scout/fixtures/dns-record-list-v1-vendor-mutation.json \
    -catalog-output "${work_root}/mutated.catalog.json" \
    -receipt-output "${work_root}/mutated.receipt.json"

go run ../cmd/campaign \
    -campaign-id network-10.4.57-dns-record-mutation \
    -profile-class fresh_seeded \
    -builder-image "${builder_image}" \
    -baseline "${catalog}" \
    -candidate "${work_root}/mutated.catalog.json" \
    -receipt "${work_root}/mutated.receipt.json" \
    -elapsed-milliseconds 0 \
    -decision "reject unreviewed vendor field" \
    "${claim_args[@]}" \
    -output ${target}/dns_record.mutation-attestation.json

jq -e '.classification == "equivalent" and .promotion == "review_required"' \
    ${target}/dns_record.attestation.json >/dev/null
jq -e '.classification == "blocked" and .promotion == "review_required"' \
    ${target}/dns_record.mutation-attestation.json >/dev/null
