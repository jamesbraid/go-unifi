# Controller scout

`go run ./cmd/scout` joins one declared controller observation to the locked
structural specification. It emits a value-free observed catalog and scenario
receipt. The command does not provision targets, crawl the UI, inspect an
undeclared controller, generate provider code, or admit operations.

Offline replay is the deterministic development path:

```sh
go run ./cmd/scout \
  -target-profile scout/profiles/network-10.4.57-seeded.json \
  -scenario scout/scenarios/dns-record-list-v1.json \
  -specification specification.json \
  -capture-lock schemas/capture.lock.json \
  -response scout/fixtures/dns-record-list-v1.json \
  -catalog-output catalogs/network-10.4.57/dns_record.catalog.json \
  -receipt-output catalogs/network-10.4.57/dns_record.scenario-receipt.json
```

Omit `-response` to execute the declared read-only DNS list against a
disposable target. Supply `UNIFI_API`, `UNIFI_USERNAME`, and `UNIFI_PASSWORD`.
`UNIFI_SITE` defaults to `default` and `UNIFI_INSECURE` defaults to `false`.
Credentials and observed values are never written to either output.
