package main

import (
	"strings"
	"testing"

	"github.com/ubiquiti-community/go-unifi/internal/fields"
)

// TestNewFieldInfoRefusesANoteWhereAValidatorBelongs pins the guard on the
// validation slot.
//
// A field the controller does not describe was carrying the words
// "non-generated field" where its validator goes. Everything downstream
// treats that slot as a regex, so the sentence was published three times
// over: as the field's trailing comment, as its entry in
// FieldValidationPatterns and FieldConstraints, and -- worst -- as a
// generated Terraform validator, RegexMatches on the sentence itself, which
// accepts one value and it is not an IP address.
//
// The fix is that such a field has no validation. This keeps the note from
// coming back the next time someone wants to explain a field in the only
// string field that happens to be nearby.
func TestNewFieldInfoRefusesANoteWhereAValidatorBelongs(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a prose note was accepted as a validator")
		}
		message, _ := recovered.(string)
		if !strings.Contains(message, "non-generated field") ||
			!strings.Contains(message, "doc") {
			t.Errorf("the panic does not say what to do instead: %v", recovered)
		}
	}()

	NewFieldInfo("LastIP", "last_ip", fields.String, "non-generated field", true, false, false, "")
}

// TestNewFieldInfoAcceptsARealValidator is the control: the guard must not
// reject an ordinary pattern.
func TestNewFieldInfoAcceptsARealValidator(t *testing.T) {
	got := NewFieldInfo("MAC", "mac", fields.String, "^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$", true, false, false, "")
	if got.FieldValidation == "" {
		t.Error("a real validator was dropped")
	}
}

// TestNewFieldInfoAcceptsNoValidator is the other control: a field the
// controller does not describe carries nothing, which is how Device.MAC has
// always done it.
func TestNewFieldInfoAcceptsNoValidator(t *testing.T) {
	got := NewFieldInfo("DisplayName", "display_name", fields.String, "", true, false, false, "")
	if got.FieldValidation != "" {
		t.Errorf("FieldValidation = %q, want empty", got.FieldValidation)
	}
}
