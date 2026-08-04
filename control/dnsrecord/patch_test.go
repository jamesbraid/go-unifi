package dnsrecord

import (
	"reflect"
	"testing"
)

func TestBuildPatchVectors(t *testing.T) {
	base := Model{
		Identity:   "dns-record-1",
		Revision:   "revision-7",
		Enabled:    Set(true),
		Key:        Set("router.example"),
		Port:       Set[int64](53),
		Priority:   Set[int64](10),
		RecordType: Set("A"),
		TTL:        Set[int64](300),
		Value:      Set("192.0.2.10"),
		Weight:     Set[int64](5),
	}

	tests := []struct {
		name   string
		model  Model
		intent Intent
		want   Patch
	}{
		{
			name:   "set",
			model:  base,
			intent: Intent{Identity: base.Identity, TTL: Set[int64](0)},
			want: Patch{
				Identity: base.Identity, ExpectedRevision: base.Revision,
				TTL: Set[int64](0),
			},
		},
		{
			name:   "clear",
			model:  base,
			intent: Intent{Identity: base.Identity, Port: Clear[int64]()},
			want: Patch{
				Identity: base.Identity, ExpectedRevision: base.Revision,
				Port: Clear[int64](),
			},
		},
		{
			name:   "omit",
			model:  base,
			intent: Intent{Identity: base.Identity, Value: Omit[string]()},
			want:   Patch{Identity: base.Identity, ExpectedRevision: base.Revision},
		},
		{
			name: "default preservation",
			model: Model{
				Identity: base.Identity,
				Revision: base.Revision,
				TTL:      Omit[int64](),
			},
			intent: Intent{Identity: base.Identity},
			want:   Patch{Identity: base.Identity, ExpectedRevision: base.Revision},
		},
		{
			name:   "no-op",
			model:  base,
			intent: Intent{Identity: base.Identity, Enabled: Set(true), TTL: Set[int64](300)},
			want:   Patch{Identity: base.Identity, ExpectedRevision: base.Revision},
		},
		{
			name: "already clear",
			model: Model{
				Identity: base.Identity,
				Revision: base.Revision,
				Port:     Clear[int64](),
			},
			intent: Intent{Identity: base.Identity, Port: Clear[int64]()},
			want:   Patch{Identity: base.Identity, ExpectedRevision: base.Revision},
		},
		{
			name: "clear differs from omitted observation",
			model: Model{
				Identity: base.Identity,
				Revision: base.Revision,
				Port:     Omit[int64](),
			},
			intent: Intent{Identity: base.Identity, Port: Clear[int64]()},
			want: Patch{
				Identity: base.Identity, ExpectedRevision: base.Revision,
				Port: Clear[int64](),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := BuildPatch(test.model, test.intent)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("BuildPatch() = %#v, want %#v", got, test.want)
			}
			if got.Noop() != (test.want == (Patch{Identity: test.model.Identity, ExpectedRevision: test.model.Revision})) {
				t.Fatalf("Noop() = %v for %#v", got.Noop(), got)
			}
		})
	}
}

func TestBuildPatchRejectsIdentityAndPresenceErrors(t *testing.T) {
	tests := []struct {
		name   string
		model  Model
		intent Intent
	}{
		{name: "missing model identity", model: Model{}, intent: Intent{}},
		{name: "mismatched intent identity", model: Model{Identity: "dns-1"}, intent: Intent{Identity: "dns-2"}},
		{name: "invalid model presence", model: Model{Identity: "dns-1", TTL: Field[int64]{Presence: Presence("invalid")}}, intent: Intent{Identity: "dns-1"}},
		{name: "invalid intent presence", model: Model{Identity: "dns-1"}, intent: Intent{Identity: "dns-1", TTL: Field[int64]{Presence: Presence("invalid")}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := BuildPatch(test.model, test.intent); err == nil {
				t.Fatal("BuildPatch() succeeded")
			}
		})
	}
}

func TestOperationArtifactDocumentsSafetyContract(t *testing.T) {
	artifact := NormalizedOperation()
	if artifact.Identity == "" || artifact.CatalogIdentity == "" {
		t.Fatalf("operation identity is incomplete: %#v", artifact)
	}
	for name, value := range map[string]string{
		"idempotency": artifact.Idempotency,
		"retry":       artifact.Retry,
		"convergence": artifact.Convergence,
		"concurrency": artifact.Concurrency,
		"errors":      artifact.Errors,
	} {
		if value == "" {
			t.Fatalf("operation %s semantics are undocumented", name)
		}
	}
	if got, want := OperationDigest(), "199c002d9a1229aa7e43a94b12da6f33c7ce612ecb471eebcd6896d985b161f8"; got != want {
		t.Fatalf("OperationDigest() = %q, want %q", got, want)
	}
	artifact.Identity = "tampered"
	if NormalizedOperation().Identity == artifact.Identity {
		t.Fatal("caller mutation changed the normalized operation identity")
	}
}
