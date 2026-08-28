package unifi

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// siteCommandOwner is the one file allowed to compose a command path.
const siteCommandOwner = "site_command.go"

// TestSiteCommandsShareOnePath fails when a file other than site_command.go
// builds a command path itself.
//
// The prefix is the whole reason this exists. A relative path is joined onto
// "/" on a classic controller and onto "/proxy/network" on UniFi OS, so
// "s/%s/cmd/firewall" reaches neither, and the firewall reorder answered 404
// from the day it was written until it was found five years later. Six other
// call sites had spelled the same path correctly, which is exactly why
// nobody looked.
//
// A composition that lives in one place is either right for every caller or
// wrong for every caller, and the second kind gets noticed.
func TestSiteCommandsShareOnePath(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the package directory: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || name == siteCommandOwner ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++

		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil || !strings.Contains(value, "cmd/") {
				return true
			}
			t.Errorf("%s composes the command path %q; call c.siteCommand instead, "+
				"which owns the path and the api/ prefix", name, value)
			return true
		})
	}

	if checked == 0 {
		t.Fatal("no files were scanned; the directory walk found nothing")
	}
}

// TestSiteCommandPath pins what the helper actually sends, since every
// command in the package now depends on it.
func TestSiteCommandPath(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/cmd/") {
			gotPath = r.URL.Path
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"result":{"success":true}}]}`))
	}))
	t.Cleanup(srv.Close)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	var resp commandResult
	if err := c.siteCommand(context.Background(), "default", "firewall", map[string]any{"cmd": "reorder"}, &resp); err != nil {
		t.Fatalf("siteCommand: %v", err)
	}
	// The client is talking to a new-style controller here, so the api path
	// sits under the console's network prefix.
	if want := "/proxy/network/api/s/default/cmd/firewall"; gotPath != want {
		t.Errorf("command went to %q, want %q", gotPath, want)
	}
}

// TestReorderFirewallRulesRejectsASilentNoOp covers the half of the defect a
// corrected path does not fix. Measured on 10.6.101: a command the
// controller does not recognize answers HTTP 200, rc ok, and an empty data
// array. The old code passed nil for the response and returned success for
// exactly that.
func TestReorderFirewallRulesRejectsASilentNoOp(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantErr    string
	}{
		{"unrecognized command", `{"meta":{"rc":"ok"},"data":[]}`, "did nothing"},
		{"reported unsuccessful", `{"meta":{"rc":"ok"},"data":[{"result":{"success":false}}]}`, "unsuccessful"},
		{"carried out", `{"meta":{"rc":"ok"},"data":[{"result":{"success":true}}]}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if handleNewStyleSetup(w, r) {
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			err = c.ReorderFirewallRules(context.Background(), "default", "LAN_IN",
				[]FirewallRuleIndexUpdate{{Id: "a", RuleIndex: 2001}})
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("a carried-out reorder returned %v", err)
			case tc.wantErr != "" && err == nil:
				t.Errorf("a reorder the controller did not perform reported success")
			case tc.wantErr != "" && err != nil && !strings.Contains(err.Error(), tc.wantErr):
				t.Errorf("error %q does not mention %q", err, tc.wantErr)
			}
		})
	}
}

// TestReorderFirewallRulesSendsTheRulesetAndRules pins the payload, which
// the controller validates: naming a ruleset the rules are not in answers
// api.err.FirewallRuleIndexNotExists.
func TestReorderFirewallRulesSendsTheRulesetAndRules(t *testing.T) {
	var got map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if handleNewStyleSetup(w, r) {
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[{"result":{"success":true}}]}`))
	}))
	t.Cleanup(srv.Close)

	c, err := New(context.Background(), &Config{BaseURL: srv.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ReorderFirewallRules(context.Background(), "default", "LAN_IN", []FirewallRuleIndexUpdate{
		{Id: "aaa", RuleIndex: 2003},
		{Id: "bbb", RuleIndex: 2001},
	}); err != nil {
		t.Fatalf("ReorderFirewallRules: %v", err)
	}

	for wire, want := range map[string]string{
		"cmd":     `"reorder"`,
		"ruleset": `"LAN_IN"`,
		// The index is carried as a string. Measured on 10.6.101: the
		// controller applies a reorder sent either way, so this is the
		// shape that shipped rather than the only one it takes.
		"rules": `[{"_id":"aaa","rule_index":"2003"},{"_id":"bbb","rule_index":"2001"}]`,
	} {
		if string(got[wire]) != want {
			t.Errorf("%s = %s, want %s", wire, got[wire], want)
		}
	}
}
