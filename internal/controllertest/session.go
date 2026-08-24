// Package controllertest boots a disposable UniFi Network controller in simulation
// mode and provides a raw HTTP session against it. The session deliberately
// returns undecoded JSON: its consumers are probes looking for fields the
// SDK's generated structs do NOT know about yet.
package controllertest

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

// ErrNotJSON is returned by GetJSON when the response body cannot be
// decoded as JSON at all (e.g. the controller's boot-time HTML
// placeholder page). It is distinct from a legal JSON `null` body, which
// GetJSON reports as (nil, status, nil) — callers that need to tell the
// two apart should check errors.Is(err, ErrNotJSON).
var ErrNotJSON = errors.New("response body is not JSON")

// Session is a cookie-authenticated raw client for a UniFi controller
// (self-signed TLS accepted). It speaks both auth dialects: the classic
// controller's /api/login, and UniFi OS's SSO, which additionally requires a
// CSRF header on every write.
type Session struct {
	baseURL string
	// rootURL is the console root for a UniFi OS target, where the SSO login
	// endpoint lives. baseURL there points into the /proxy/network prefix, so
	// the two differ; for a classic controller rootURL is empty.
	rootURL string
	// csrf is the UniFi OS CSRF token minted at login. Empty for classic
	// sessions, which need no such header.
	csrf   string
	client *http.Client
}

func NewSession(baseURL string) *Session {
	jar, _ := cookiejar.New(nil)
	return &Session{
		baseURL: baseURL,
		client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // local throwaway controller
			},
		},
	}
}

// NewUOSSession returns a session for a UniFi OS console. baseURL is the
// Network API behind the console proxy (…/proxy/network); rootURL is the
// console itself, where SSO login lives.
func NewUOSSession(rootURL, baseURL string) *Session {
	s := NewSession(baseURL)
	s.rootURL = rootURL
	return s
}

// LoginUOS authenticates against UniFi OS SSO. It keeps the TOKEN cookie in
// the jar and the CSRF token the console returns, which every subsequent write
// must echo back — without it UniFi OS answers 403 Forbidden.
//
// Call this ONCE per session. UniFi OS rate-limits the login endpoint (HTTP
// 429 "You've reached the login attempt limit") after a handful of attempts,
// counting successful ones, and the block then applies for minutes. A test
// that re-authenticates per request locks itself out and the failure surfaces
// as unrelated 401s on later calls.
func (s *Session) LoginUOS(ctx context.Context, username, password string) error {
	creds, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.rootURL+"/api/auth/login", bytes.NewReader(creds))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("UniFi OS login rate-limited (HTTP 429): log in once per session")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("UniFi OS login returned HTTP %s", resp.Status)
	}

	// The console returns the token under either spelling depending on
	// whether it minted a fresh one or refreshed the current one.
	csrf := resp.Header.Get("X-Csrf-Token")
	if csrf == "" {
		csrf = resp.Header.Get("X-Updated-Csrf-Token")
	}
	if csrf == "" {
		return errors.New("UniFi OS login returned no CSRF token")
	}
	s.csrf = csrf
	return nil
}

// Login authenticates against the classic /api/login endpoint; the session
// cookie is kept in the jar. HTTP 200 alone is not enough: while booting,
// the controller answers every path — including /api/login — with a 200
// HTML "Server status" placeholder page, so success additionally requires
// a JSON body whose meta.rc is "ok".
func (s *Session) Login(ctx context.Context, username, password string) error {
	creds, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/login", bytes.NewReader(creds))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned HTTP %s", resp.Status)
	}

	var body struct {
		Meta struct {
			RC string `json:"rc"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return fmt.Errorf("login returned a non-JSON body (controller may still be booting): %w", ErrNotJSON)
	}
	if body.Meta.RC != "ok" {
		return fmt.Errorf("login returned meta.rc %q", body.Meta.RC)
	}
	return nil
}

// GetJSON fetches path and returns the decoded body, the HTTP status code,
// and any error. Non-2xx statuses are not errors: probes need to
// distinguish 404 (endpoint absent in this controller version) from
// failure. A body that fails to decode as JSON returns (nil, status,
// ErrNotJSON); a legal JSON `null` body returns (nil, status, nil) — the
// two are not the same thing, and callers that care (e.g. the drift probe)
// must use errors.Is to tell them apart.
func (s *Session) GetJSON(ctx context.Context, path string) (any, int, error) {
	return s.do(ctx, http.MethodGet, path, nil)
}

// PostJSON sends a JSON body; same return convention as GetJSON (non-2xx
// statuses are results, not errors — probes classify them).
func (s *Session) PostJSON(ctx context.Context, path string, body any) (any, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	return s.do(ctx, http.MethodPost, path, payload)
}

// DeleteJSON deletes path; same return convention as GetJSON.
func (s *Session) DeleteJSON(ctx context.Context, path string) (any, int, error) {
	return s.do(ctx, http.MethodDelete, path, nil)
}

// PutJSON sends a JSON body via PUT; same return convention as GetJSON. The
// classic controller API updates site-wide settings objects (as opposed to
// rest/* collections, which use POST) at /api/s/{site}/set/setting/{key}.
func (s *Session) PutJSON(ctx context.Context, path string, body any) (any, int, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	return s.do(ctx, http.MethodPut, path, payload)
}

func (s *Session) do(ctx context.Context, method, path string, payload []byte) (any, int, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// UniFi OS rejects every write that does not echo the login's CSRF token.
	if s.csrf != "" {
		req.Header.Set("X-Csrf-Token", s.csrf)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, resp.StatusCode, ErrNotJSON
	}
	return decoded, resp.StatusCode, nil
}
