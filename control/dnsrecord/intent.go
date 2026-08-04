package dnsrecord

// Presence describes whether a field is preserved, explicitly cleared, or
// explicitly set. Its zero value is omit so a sparse Intent preserves
// controller defaults.
type Presence string

const (
	PresenceOmit  Presence = ""
	PresenceClear Presence = "clear"
	PresenceSet   Presence = "set"
)

// Field retains presence independently from its value. In particular, Set(0)
// and Clear[int64]() are different requests.
type Field[T comparable] struct {
	Presence Presence
	Value    T
}

// Omit preserves the observed field and any controller default.
func Omit[T comparable]() Field[T] { return Field[T]{Presence: PresenceOmit} }

// Clear requests explicit removal of the field's value.
func Clear[T comparable]() Field[T] { return Field[T]{Presence: PresenceClear} }

// Set requests an explicit value, including the type's zero value.
func Set[T comparable](value T) Field[T] { return Field[T]{Presence: PresenceSet, Value: value} }

func (field Field[T]) valid() bool {
	switch field.Presence {
	case PresenceOmit, PresenceClear:
		var zero T
		return field.Value == zero
	case PresenceSet:
		return true
	default:
		return false
	}
}

// Intent is the desired DNS record state. Omitted fields are preserved rather
// than materialized from controller defaults.
type Intent struct {
	Identity string

	Enabled    Field[bool]
	Key        Field[string]
	Port       Field[int64]
	Priority   Field[int64]
	RecordType Field[string]
	TTL        Field[int64]
	Value      Field[string]
	Weight     Field[int64]
}
