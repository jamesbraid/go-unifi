# Controller scout

`go run ./cmd/scout` joins one declared raw controller observation to the
locked, policy-free structural projection and reviewed semantic IDs. It emits a
value-free observed catalog and scenario receipt. The command does not
provision targets, crawl the UI, inspect an undeclared controller, generate
provider code, or admit operations.

Offline replay is the deterministic development path:

```sh
go run ./cmd/scout \
  -target-profile scout/profiles/network-10.4.57-seeded.json \
  -scenario scout/scenarios/dns-record-list-v1.json \
  -structural schemas/structural/dns_record.json \
  -semantic-predecessor schemas/semantic-ids/dns_record.previous.json \
  -semantic-ids schemas/semantic-ids/dns_record.json \
  -capture-lock schemas/capture.lock.json \
  -response scout/fixtures/dns-record-list-v1.json \
  -catalog-output catalogs/network-10.4.57/dns_record.catalog.json \
  -receipt-output catalogs/network-10.4.57/dns_record.scenario-receipt.json
```

Omit `-response` to execute the declared read-only DNS list against a
disposable target. Live runs also require `-target-receipt` with the
provisioner's measured product, version, architecture, image digests, and a
SHA-256 digest of the runtime instance identity. That identity digest is
`sha256:` followed by the SHA-256 of the exact UTF-8 controller UUID reported
by the status endpoint. Scout computes the receipt's fingerprint and requires
it to match the target profile. It independently reads the Network version and
controller UUID learned by the API client. The version must match both the
profile and capture lock, and the UUID digest must match the provisioner
receipt. A controller that supplies a version through the sysinfo fallback but
no status UUID cannot produce live evidence.

Supply `UNIFI_API`, `UNIFI_USERNAME`, and `UNIFI_PASSWORD`. `UNIFI_SITE`
defaults to `default` and `UNIFI_INSECURE` defaults to `false`. The live request
retains raw JSON through admission checks. Fixture runs forbid target receipts
and carry no measured-target claim. Credentials, instance names, and observed
values are never written to either output.
