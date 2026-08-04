package dnsrecord

import "fmt"

// Patch is a transport-neutral set of presence-aware DNS field changes.
// ExpectedRevision is an optimistic concurrency precondition when the
// controller exposes one; adapters must not silently discard a non-empty one.
type Patch struct {
	Identity         string
	ExpectedRevision string

	Enabled    Field[bool]
	Key        Field[string]
	Port       Field[int64]
	Priority   Field[int64]
	RecordType Field[string]
	TTL        Field[int64]
	Value      Field[string]
	Weight     Field[int64]
}

// Noop reports whether the patch preserves every field.
func (patch Patch) Noop() bool {
	return patch.Enabled.Presence == PresenceOmit &&
		patch.Key.Presence == PresenceOmit &&
		patch.Port.Presence == PresenceOmit &&
		patch.Priority.Presence == PresenceOmit &&
		patch.RecordType.Presence == PresenceOmit &&
		patch.TTL.Presence == PresenceOmit &&
		patch.Value.Presence == PresenceOmit &&
		patch.Weight.Presence == PresenceOmit
}

// BuildPatch converts one observed Model and desired Intent into the minimal
// presence-aware Patch. It performs no I/O and selects no transport.
func BuildPatch(model Model, intent Intent) (Patch, error) {
	if model.Identity == "" {
		return Patch{}, fmt.Errorf("DNS record model identity is required")
	}
	if intent.Identity == "" || intent.Identity != model.Identity {
		return Patch{}, fmt.Errorf("DNS record intent identity does not match model")
	}
	if err := validateModel(model); err != nil {
		return Patch{}, err
	}
	if err := validateIntent(intent); err != nil {
		return Patch{}, err
	}

	return Patch{
		Identity:         model.Identity,
		ExpectedRevision: model.Revision,
		Enabled:          changedField(model.Enabled, intent.Enabled),
		Key:              changedField(model.Key, intent.Key),
		Port:             changedField(model.Port, intent.Port),
		Priority:         changedField(model.Priority, intent.Priority),
		RecordType:       changedField(model.RecordType, intent.RecordType),
		TTL:              changedField(model.TTL, intent.TTL),
		Value:            changedField(model.Value, intent.Value),
		Weight:           changedField(model.Weight, intent.Weight),
	}, nil
}

func changedField[T comparable](observed, desired Field[T]) Field[T] {
	if desired.Presence == PresenceOmit || desired == observed {
		return Omit[T]()
	}
	return desired
}

func validateModel(model Model) error {
	if !model.Enabled.valid() || !model.Key.valid() || !model.Port.valid() ||
		!model.Priority.valid() || !model.RecordType.valid() || !model.TTL.valid() ||
		!model.Value.valid() || !model.Weight.valid() {
		return fmt.Errorf("DNS record model contains invalid field presence")
	}
	return nil
}

func validateIntent(intent Intent) error {
	if !intent.Enabled.valid() || !intent.Key.valid() || !intent.Port.valid() ||
		!intent.Priority.valid() || !intent.RecordType.valid() || !intent.TTL.valid() ||
		!intent.Value.valid() || !intent.Weight.valid() {
		return fmt.Errorf("DNS record intent contains invalid field presence")
	}
	return nil
}
