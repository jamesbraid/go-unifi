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
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"time"
)

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
// cookie is kept in the jar.
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
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login returned HTTP %s", resp.Status)
	}
	return nil
}

// GetJSON fetches path and returns the decoded body (nil when the body is
// not JSON), the HTTP status code, and any transport error. Non-2xx statuses
// are not errors: probes need to distinguish 404 (endpoint absent in this
// controller version) from failure.
func (s *Session) GetJSON(ctx context.Context, path string) (any, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	var body any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, resp.StatusCode, nil
	}
	return body, resp.StatusCode, nil
}
