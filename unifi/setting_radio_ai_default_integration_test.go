//go:build integration

package unifi

import (
	"testing"
	"time"

	"github.com/ubiquiti-community/go-unifi/internal/controllertest"
)

// TestIntegrationSettingRadioAiDefaultDoesNotTrackCustomisation pins a
// negative measurement: setting_radio_ai's own "default" member -- in the
// vendor schema, not hand-added, and already generated as
// unifi/settings.RadioAi.Default -- is not a live "is this still the
// factory-default profile" flag.
//
// A downstream consumer hand-reads that state today, almost certainly by
// diffing the whole section against its own copy of the factory values,
// because "default" looked like exactly the field that should answer it
// for free. Measured on 10.6.101: it does not. A fresh site starts with
// default=true, and editing a real field (auto_channel_presets_type)
// leaves it true -- so it is not "has this section been edited away from
// its factory values", and the consumer's heuristic has nothing here to
// replace it with.
//
// This is recorded as a pinned test rather than a schemas/behavior.json
// entry for the same reason TestPortProfileDoesNotModelTaggedNetworks is a
// plain test and not an artifact row: it is a negative fact about one
// field, not a member of a family the artifact already has a shape for.
// If the controller ever starts flipping "default" on edit, that is worth
// knowing -- go tell the consumer -- so this fails loudly instead of
// passing silently through the change.
func TestIntegrationSettingRadioAiDefaultDoesNotTrackCustomisation(t *testing.T) {
	ctx, c, s := controllertest.MutatingHarness(t, 10*time.Minute)

	read := func() map[string]any {
		body, status, err := s.GetJSON(ctx, "/api/s/"+c.Site+"/get/setting/radio_ai")
		if err != nil || status != 200 {
			t.Fatalf("get/setting/radio_ai (HTTP %d): %v", status, err)
		}
		return firstData(t, body)
	}

	before := read()
	beforeDefault, ok := before["default"].(bool)
	if !ok {
		t.Fatalf(`setting_radio_ai carries no boolean "default" member on this controller: %s`, jsonText(before))
	}
	t.Logf("fresh site: setting_radio_ai.default = %v", beforeDefault)
	if !beforeDefault {
		t.Skip("this controller's fresh site did not start with default=true; the measurement this " +
			"test pins assumed that starting point and needs re-doing against this controller")
	}

	// Change one real field away from whatever the factory value is.
	// "default" is deliberately not named here: it is not in the generated
	// struct's writable set, and the question is whether the controller
	// moves it on its own the way it owns other read-only markers.
	edited := map[string]any{
		"key":                       "radio_ai",
		"auto_channel_presets_type": "conservative",
	}
	if v, _ := before["auto_channel_presets_type"].(string); v == "conservative" {
		edited["auto_channel_presets_type"] = "maximum_speed"
	}
	body, status, err := s.PutJSON(ctx, "/api/s/"+c.Site+"/set/setting/radio_ai", edited)
	if err != nil || status != 200 {
		t.Fatalf("set/setting/radio_ai (HTTP %d): %v %v", status, body, err)
	}

	after := read()
	if got, _ := after["auto_channel_presets_type"].(string); got != edited["auto_channel_presets_type"] {
		t.Fatalf("auto_channel_presets_type = %q after the write, want %q; the edit did not take, "+
			"so this run cannot say anything about default", got, edited["auto_channel_presets_type"])
	}
	afterDefault, ok := after["default"].(bool)
	if !ok {
		t.Fatalf(`setting_radio_ai lost its "default" member after a write: %s`, jsonText(after))
	}
	t.Logf("after editing auto_channel_presets_type to %v: setting_radio_ai.default = %v",
		edited["auto_channel_presets_type"], afterDefault)

	if afterDefault != beforeDefault {
		t.Errorf(`setting_radio_ai.default changed from %v to %v after editing a real field.`+"\n\n"+
			`This test exists to PIN the measured fact that it does not -- the controller changed, `+
			`which means "default" may now be usable as the factory-default marker a downstream `+
			`consumer hand-reads today. Re-measure before touching that consumer.`,
			beforeDefault, afterDefault)
	}
}
