package controllertest

import (
	"os"
	"strings"
	"testing"
)

// TestDefaultImageMatchesTheCapturedVersion ties the local-run default to the
// controller the schemas were captured from.
//
// The two drifted: the constant sat at 10.4.57-sim while schemas/VERSION moved
// to 10.6.101, so a bare local run started a controller two releases behind
// the SDK under test. CI derives its tag from the marker and was unaffected,
// which is why nothing caught it -- and why the existing check could not: it
// compares imageFromEnv() to defaultImage, so it holds whatever the constant
// says.
//
// This matters most for the field probe. A verdict of STRIPPED or PERSISTED is
// a statement about one controller generation, and measuring it against the
// wrong one attributes the answer to a release that never produced it.
func TestDefaultImageMatchesTheCapturedVersion(t *testing.T) {
	raw, err := os.ReadFile("../../schemas/VERSION")
	if err != nil {
		t.Fatalf("read the captured version: %v", err)
	}
	version := strings.TrimSpace(string(raw))
	if version == "" {
		t.Fatal("schemas/VERSION is empty; this check would pass against anything")
	}

	want := "ghcr.io/jamesbraid/unifi-network:" + version + "-sim"
	if defaultImage != want {
		t.Errorf("defaultImage = %q, want %q\n\n"+
			"schemas/VERSION is %s, so a bare local run must start that controller. "+
			"CI derives its tag from the marker and will not notice the difference; "+
			"a local measurement attributed to the wrong generation is the failure "+
			"this guards.", defaultImage, want, version)
	}
}
