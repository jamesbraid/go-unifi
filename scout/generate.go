package scout

//go:generate go run ../cmd/scout -target-profile profiles/network-10.4.57-seeded.json -scenario scenarios/dns-record-list-v1.json -structural ../schemas/structural/dns_record.json -semantic-ids ../schemas/semantic-ids/dns_record.json -capture-lock ../schemas/capture.lock.json -response fixtures/dns-record-list-v1.json -catalog-output ../catalogs/network-10.4.57/dns_record.catalog.json -receipt-output ../catalogs/network-10.4.57/dns_record.scenario-receipt.json
