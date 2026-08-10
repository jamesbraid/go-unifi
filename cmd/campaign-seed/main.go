// Command campaign-seed puts one static DNS record on the campaign's disposable
// controller so the scout has something to observe.
//
// It exists because the live campaign kept classifying a perfectly good capture
// as capture_invalid: the scout performs one read-only GET against
// /v2/api/site/{site}/static-dns, and a freshly provisioned controller has no
// static DNS records at all. The response was an empty collection, so all eight
// declared fields came back unexercised, admission blocked, and the classifier
// reported an invalid capture. Nothing was broken. There was simply nothing
// there.
//
// THE SEEDING IS DELIBERATELY NOT IN THE SCOUT. cmd/scout refuses any scenario
// that is not the declared GET, and internal/scout/catalog.go refuses any
// scenario whose mode is not read_only -- which is why the catalog can record
// cleanupPolicy "not_required_read_only". That the scout cannot mutate its
// target is a property worth keeping, not a limitation to route around: it is
// what makes a campaign safe to point at a controller someone cares about.
// So the campaign writes and the scout reads, and the two stay separable.
//
// Every declared field is populated on purpose. Most are `omitempty`, so a
// record that leaves one at its zero value does not serialize it, the
// controller never returns it, and admission blocks on that field alone -- the
// same failure with more steps. An SRV record is the one type that carries
// port, priority and weight alongside the rest.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ubiquiti-community/go-unifi/unifi"
)

// seedKey is distinctive so a human finding this record on a controller knows
// what put it there and that it is disposable.
const seedKey = "_campaign._tcp.seed.invalid"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "campaign-seed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	api, username, password := os.Getenv("UNIFI_API"), os.Getenv("UNIFI_USERNAME"), os.Getenv("UNIFI_PASSWORD")
	if api == "" || username == "" || password == "" {
		return errors.New("UNIFI_API, UNIFI_USERNAME and UNIFI_PASSWORD are required")
	}
	site := os.Getenv("UNIFI_SITE")
	if site == "" {
		site = "default"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, err := unifi.New(ctx, &unifi.Config{
		BaseURL: api, Username: username, Password: password, AllowInsecure: true,
	})
	if err != nil {
		return fmt.Errorf("connect to the controller: %w", err)
	}

	existing, err := client.ListDNSRecord(ctx, site)
	if err != nil {
		return fmt.Errorf("list static DNS records: %w", err)
	}
	for _, record := range existing {
		if record.Key == seedKey {
			fmt.Printf("seed already present on site %q (%s); leaving it alone\n", site, record.ID)
			return verify(ctx, client, site)
		}
	}
	if len(existing) > 0 {
		fmt.Printf("note: site %q already holds %d static DNS record(s) not placed by this command\n", site, len(existing))
	}

	port := int64(5060)
	created, err := client.CreateDNSRecord(ctx, site, &unifi.DNSRecord{
		Enabled:    true,
		Key:        seedKey,
		RecordType: "SRV",
		Value:      "sip.seed.invalid",
		Port:       &port,
		Priority:   10,
		Weight:     20,
		Ttl:        3600,
	})
	if err != nil {
		return fmt.Errorf("create the seed static DNS record: %w", err)
	}
	fmt.Printf("seeded one SRV static DNS record on site %q (%s)\n", site, created.ID)
	return verify(ctx, client, site)
}

// verify reads the record back and insists every declared field came with it.
// Creating something the controller then does not report is the exact failure
// this command exists to remove, and it would otherwise look identical to not
// having seeded at all.
func verify(ctx context.Context, client *unifi.ApiClient, site string) error {
	records, err := client.ListDNSRecord(ctx, site)
	if err != nil {
		return fmt.Errorf("read the seed back: %w", err)
	}
	for _, record := range records {
		if record.Key != seedKey {
			continue
		}
		var missing []string
		if !record.Enabled {
			missing = append(missing, "enabled")
		}
		if record.Key == "" {
			missing = append(missing, "key")
		}
		if record.Port == nil || *record.Port == 0 {
			missing = append(missing, "port")
		}
		if record.Priority == 0 {
			missing = append(missing, "priority")
		}
		if record.RecordType == "" {
			missing = append(missing, "record_type")
		}
		if record.Ttl == 0 {
			missing = append(missing, "ttl")
		}
		if record.Value == "" {
			missing = append(missing, "value")
		}
		if record.Weight == 0 {
			missing = append(missing, "weight")
		}
		if len(missing) > 0 {
			return fmt.Errorf("the controller did not return %v for the seeded record, so those fields stay unexercised and admission will still block", missing)
		}
		fmt.Println("verified: the controller returns all eight declared fields for the seed")
		return nil
	}
	return fmt.Errorf("the seeded record is not in the collection the scout will read")
}
