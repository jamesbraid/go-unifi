// Package testenv boots a disposable UniFi Network controller in simulation
// mode and provides a raw HTTP session against it. The session deliberately
// returns undecoded JSON: its consumers are probes looking for fields the
// SDK's generated structs do NOT know about yet.
package testenv

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

// Session is a cookie-authenticated raw client for a classic UniFi
// controller (self-signed TLS accepted).
type Session struct {
	baseURL string
	client  *http.Client
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
		return fmt.Errorf("login returned a non-JSON body (controller may still be booting): %w", err)
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
