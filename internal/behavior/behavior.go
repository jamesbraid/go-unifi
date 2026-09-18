// Package behavior holds the measured-behaviour artifact: what the pinned
// controller actually does, recorded by probes at capture time and versioned
// beside the field definitions it complements.
//
// The field definitions say what the controller declares; this says what it
// was measured doing -- which fields a preference mode silently owns, which
// writes it accepts and discards, and whether an absent key and an empty
// string mean the same thing. Every entry here used to be a hand-pasted
// baseline inside an integration test, stamped with a controller version and
// forgotten until it went red. The probes write this file, the tests read it,
// and a controller bump re-measures it into a reviewable diff.
package behavior

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Artifact is the whole measured-behaviour record for one controller version.
// Maps are keyed for stable diffs; MarshalJSON is not needed because encoding/
// json sorts map keys, and the writer indents.
type Artifact struct {
	// ControllerVersion is the version the probes ran against, so a stale
	// artifact is obvious in the diff rather than silently trusted.
	ControllerVersion string `json:"controller_version"`

	// Ownership: per resource, per preference mode field, the wire names the
	// controller silently manages when that mode is set -- accepted on write,
	// stored as the controller's own value, never reported. Measured on the
	// standalone controller; the generator derives
	// unifi/preference.generated.go from this.
	Ownership map[string]map[string][]string `json:"ownership,omitempty"`

	// UOSPins: keyed like Ownership, the subset of a mode's owns the UniFi
	// OS console stores its own value for under BOTH modes -- fields neither
	// mode reaches inside UniFi OS. The ownership sweep records them when it
	// runs on the UOS harness; the generator publishes them as
	// Preference.UOSExcludes. A key absent here has no measured exclusions.
	UOSPins map[string]map[string][]string `json:"uos_pins,omitempty"`

	// Discarded: per resource, wire names the controller accepts on create
	// and does not store -- the round-trip probe's finding. Replaces the
	// wantDiscarded baselines.
	Discarded map[string][]string `json:"discarded,omitempty"`

	// Empty: per resource, per field, how the controller treats an empty
	// string versus an absent key, in the clearing probe's vocabulary.
	//
	// One entry per field is only ever true when every branch of the
	// resource agrees. A field's disposition can turn on a discriminator
	// the same way a write contract's required set can (see
	// WriteContract.RequiredOnCreateWhen) -- nat.ip_address is EMPTY-
	// REJECTED on every type, but OMIT-REJECTED on DNAT and SNAT while
	// MASQUERADE accepts an absent key unconditionally because the field
	// can never hold a value there at all. Flattening that disagreement
	// into one branch's answer -- which is how nat.ip_address ended up
	// pinned OMIT-REJECTED, measured on SNAT alone -- tells the other
	// branch's callers something false. When EmptyWhen carries branches
	// for a field, this map holds only what every one of them agrees on,
	// so a consumer reading Empty alone is told less, never told wrong.
	Empty map[string]map[string]EmptySemantics `json:"empty,omitempty"`

	// EmptyWhen carries the Empty verdicts that hold only for part of a
	// resource, mirroring WriteContract.RequiredOnCreateWhen: keyed by
	// resource, then by field, then by the discriminator branch that
	// selects the verdict -- a comma-separated list of wire field and
	// value, sorted by field name ("type=MASQUERADE"), the same branch-key
	// spelling RequiredOnCreateWhen uses.
	//
	// A field absent from this map was measured the same across every
	// branch tried (or only one branch was ever measurable), and Empty
	// alone already says what that is. A field present here disagreed
	// across branches, and Empty holds only the intersection -- read this
	// map for the rest.
	EmptyWhen map[string]map[string]map[string]EmptySemantics `json:"empty_when,omitempty"`

	// Coercions: per resource, per field, a below-range value that was
	// written and the value the controller floored/clamped it to -- the
	// conntrack timeout floors and their kin.
	Coercions map[string]map[string]Coercion `json:"coercions,omitempty"`

	// Writes: per resource, the measured write contract -- the verb and path
	// the controller actually accepts, and which fields must be present on
	// create and on update. Replaces the codegen's guess-from-shape.
	Writes map[string]WriteContract `json:"writes,omitempty"`

	// UOSWrites: keyed like Writes, a write contract measured to hold only
	// on the UniFi OS harness -- the standalone controller answers the same
	// request differently (a discarded write, a crash, a rejection), so
	// filing it beside Writes would claim a create path the standalone
	// controller does not actually have. Mirrors the Ownership/UOSPins
	// split: a resource entirely absent from Writes is not "unmeasured", it
	// is "no working create path on the standalone controller", same as
	// today; this map is where a UOS-only path is recorded instead of
	// papering over the difference. DeviceTag is the first: its create
	// persists only on UOS, confirmed a no-op on the standalone harness
	// across five retries and three collection names.
	UOSWrites map[string]WriteContract `json:"uos_writes,omitempty"`

	// RejectedCreates: per resource, what a create the controller rejected
	// left behind -- the status it answered and whether the document was
	// stored anyway. A rejection that stores is why a client must never
	// replay a POST it saw fail: each replay can file another copy.
	RejectedCreates map[string]RejectedCreate `json:"rejected_creates,omitempty"`

	// Replays: per resource, what re-sending an identical write did -- the
	// measured ground under the transport's retry policy (replay PUT, never
	// POST).
	Replays map[string]Replay `json:"replays,omitempty"`

	// Capabilities: per resource, per combination of discriminator fields,
	// whether a create using that combination is accepted -- and if refused,
	// the controller's own message. Keyed like
	// WriteContract.RequiredOnCreateWhen: a comma-separated list of wire
	// field and value, sorted by field name ("ip_version=IPV4,protocol=tcp").
	//
	// This replaces a hand-maintained compatibility table with a measured
	// one: FirewallPolicy's (protocol, ip_version) matrix used to live as
	// ~60 literals in a downstream consumer, pinned by a comment naming a
	// controller version rather than anything checkable. The domain swept
	// into each key must come from the controller's own declarations -- the
	// generated Values slice for an enum field, the field's own validation
	// pattern for a pattern-typed one -- never from a list this project
	// maintains, so a controller that adds a value shows up here as a new
	// row instead of being silently skipped.
	Capabilities map[string]map[string]Capability `json:"capabilities,omitempty"`
}

// Capability is one branch's measured create outcome: accepted, or refused
// with the controller's own message. Error is empty when Accepted is true.
type Capability struct {
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}

// RejectedCreate is the controller's answer to one deliberately invalid
// create: the rejection status, and whether the document turned up in the
// collection afterwards regardless.
type RejectedCreate struct {
	Status int  `json:"status"`
	Stored bool `json:"stored"`
}

// Replay records what an identical second write did. Create is "DUPLICATES"
// (a second document with its own id) or "REJECTED-<status>"; Update is
// "IDEMPOTENT" (accepted, document unchanged) or "CHANGED-<fields>".
type Replay struct {
	Create string `json:"create,omitempty"`
	Update string `json:"update,omitempty"`
}

// EmptySemantics records what a field did when written as "" and when its key
// was omitted (EMPTY-REJECTED / EMPTY-CLEARS / EMPTY-IGNORED, OMIT-CLEARS /
// OMIT-KEEPS / OMIT-REJECTED).
type EmptySemantics struct {
	Empty string `json:"empty"`
	Omit  string `json:"omit"`
}

// Coercion is a written value the controller refused to store verbatim, with
// what it stored instead.
type Coercion struct {
	Wrote  string `json:"wrote"`
	Stored string `json:"stored"`
}

// WriteContract is the measured verb/path and required-field sets for a
// resource's create and update. Nested fields are named by dotted path
// ("source.zone_id"), like the ownership section names them.
type WriteContract struct {
	CreateVerb string `json:"create_verb"`
	CreatePath string `json:"create_path"`
	UpdateVerb string `json:"update_verb"`
	UpdatePath string `json:"update_path"`
	// RequiredOnCreate and RequiredOnUpdate say what the CONTROLLER
	// refuses, and nothing more. They are not a statement about any
	// consumer's schema: a consumer that supplies the field itself -- from
	// a default, or from another object it already holds -- can leave it
	// optional to its own users and still satisfy the controller. Compiling
	// one of these into "the user must provide this" is an inference this
	// artifact cannot support, and it has already broken a downstream
	// provider once by forcing a field its own code was filling in.
	RequiredOnCreate []string `json:"required_on_create,omitempty"`
	RequiredOnUpdate []string `json:"required_on_update,omitempty"`

	// RequiredOnCreateWhen carries the requirements that hold only for part
	// of a resource, keyed by the branch that selects them: a
	// comma-separated list of wire field and value, sorted by field name
	// ("purpose=site-vpn,vpn_type=ipsec-vpn", "security=wpaeap").
	//
	// A network's required set is not one set -- a WAN create may omit
	// everything, an IPsec site-to-site create may not -- and flattening the
	// branches into RequiredOnCreate would publish each branch's rules as
	// though every create had to obey them. When this map is populated,
	// RequiredOnCreate holds only what EVERY measured branch requires, so an
	// empty RequiredOnCreate beside a populated map reads as "nothing is
	// required unconditionally", not as "nobody measured it".
	//
	// Only RequiredOnCreate reaches the generator: a field required in one
	// branch is optional in another, and a struct tag cannot say "sometimes".
	RequiredOnCreateWhen map[string][]string `json:"required_on_create_when,omitempty"`

	// MinItems: per list field, the measured smallest length a present list
	// may carry. Distinct from RequiredOnCreate, which says whether the key
	// may be omitted at all.
	MinItems map[string]int `json:"min_items,omitempty"`
}

// Path is the artifact's location relative to the module root.
const Path = "schemas/behavior.json"

// Load reads the artifact from root. A missing file is not an error -- it
// returns a zero Artifact and false, so a consumer that predates the artifact
// degrades to "nothing measured" rather than failing.
func Load(root string) (Artifact, bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, Path))
	if os.IsNotExist(err) {
		return Artifact{}, false, nil
	}
	if err != nil {
		return Artifact{}, false, err
	}
	var a Artifact
	if err := json.Unmarshal(raw, &a); err != nil {
		return Artifact{}, false, fmt.Errorf("parse %s: %w", Path, err)
	}
	return a, true, nil
}

// Write renders the artifact to root deterministically: sorted keys (via
// encoding/json), two-space indent, trailing newline, so a re-measure that
// found nothing new is a zero-line diff.
func Write(root string, a Artifact) error {
	// sort slice values so an unordered probe result still diffs stably.
	for _, byField := range a.Ownership {
		for k := range byField {
			sort.Strings(byField[k])
		}
	}
	for _, byField := range a.UOSPins {
		for k := range byField {
			sort.Strings(byField[k])
		}
	}
	for k := range a.Discarded {
		sort.Strings(a.Discarded[k])
	}
	for _, w := range a.Writes {
		sort.Strings(w.RequiredOnCreate)
		sort.Strings(w.RequiredOnUpdate)
		for k := range w.RequiredOnCreateWhen {
			sort.Strings(w.RequiredOnCreateWhen[k])
		}
	}
	for _, w := range a.UOSWrites {
		sort.Strings(w.RequiredOnCreate)
		sort.Strings(w.RequiredOnUpdate)
		for k := range w.RequiredOnCreateWhen {
			sort.Strings(w.RequiredOnCreateWhen[k])
		}
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(filepath.Join(root, Path), raw, 0o644)
}
