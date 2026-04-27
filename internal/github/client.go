// Package github is a minimal GraphQL client for the GitHub API.
//
// We use GraphQL exclusively because it lets us bundle commit author profile
// fields (login, name, avatarUrl, bio) into the same request as the commit
// history, avoiding the 5000-call N+1 we'd hit with REST `/users/{login}`.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const apiURL = "https://api.github.com/graphql"

type Client struct {
	http    *http.Client
	token   string
	limiter *rate.Limiter

	// rate-limit accounting from the most recent response, for logging
	lastCost      int
	lastRemaining int
	lastResetAt   time.Time
}

// NewClient constructs a client that paces calls at one per minInterval to
// respect GitHub's secondary rate-limit guidance (≤2 GraphQL req/sec).
func NewClient(token string) *Client {
	return &Client{
		http:    &http.Client{Timeout: 60 * time.Second},
		token:   token,
		limiter: rate.NewLimiter(rate.Every(500*time.Millisecond), 5),
	}
}

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type graphQLError struct {
	Type    string   `json:"type,omitempty"`
	Path    []any    `json:"path,omitempty"`
	Message string   `json:"message"`
}

func (e graphQLError) Error() string { return e.Message }

type rateLimit struct {
	Cost      int       `json:"cost"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

// Do executes a GraphQL query and unmarshals the `data` field into out.
// Retries on transient (5xx, network) errors with exponential backoff.
func (c *Client) Do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: vars})
	if err != nil {
		return err
	}

	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}
		err := c.do(ctx, body, out)
		if err == nil {
			return nil
		}
		var transient transientError
		if !errors.As(err, &transient) {
			return err
		}
		lastErr = err
		backoff := time.Duration(1<<attempt) * time.Second
		slog.Warn("graphql transient error, retrying", "attempt", attempt, "backoff", backoff, "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
	return fmt.Errorf("graphql exhausted retries: %w", lastErr)
}

type transientError struct{ err error }

func (e transientError) Error() string { return e.err.Error() }
func (e transientError) Unwrap() error { return e.err }

func (c *Client) do(ctx context.Context, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "openssf-contribcard/0.1")

	resp, err := c.http.Do(req)
	if err != nil {
		return transientError{err: fmt.Errorf("http: %w", err)}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return transientError{err: fmt.Errorf("read body: %w", err)}
	}

	if resp.StatusCode >= 500 {
		return transientError{err: fmt.Errorf("http %d: %s", resp.StatusCode, truncate(respBody, 200))}
	}
	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized — check GH_TOKEN")
	}
	if resp.StatusCode == 403 {
		// Could be secondary rate limit or auth scope; treat as transient once.
		return transientError{err: fmt.Errorf("http 403: %s", truncate(respBody, 200))}
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("http %d: %s", resp.StatusCode, truncate(respBody, 400))
	}

	// GitHub returns errors at the top level alongside data. Decode both.
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w; body=%s", err, truncate(respBody, 200))
	}
	if len(envelope.Errors) > 0 {
		// Some "errors" (e.g. RATE_LIMITED) are transient; the simple thing is
		// to surface the first error message and retry once on RATE_LIMITED.
		first := envelope.Errors[0]
		if first.Type == "RATE_LIMITED" {
			return transientError{err: first}
		}
		return fmt.Errorf("graphql: %s", first.Message)
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("graphql: empty data; body=%s", truncate(respBody, 200))
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}

	// Best-effort: pull rateLimit out of the data object if present.
	var rl struct {
		RateLimit *rateLimit `json:"rateLimit"`
	}
	if err := json.Unmarshal(envelope.Data, &rl); err == nil && rl.RateLimit != nil {
		c.lastCost = rl.RateLimit.Cost
		c.lastRemaining = rl.RateLimit.Remaining
		c.lastResetAt = rl.RateLimit.ResetAt
	}
	return nil
}

// LastRateLimit returns the cost/remaining/reset reported by the most recent
// successful response. Zero values if none seen yet.
func (c *Client) LastRateLimit() (cost, remaining int, resetAt time.Time) {
	return c.lastCost, c.lastRemaining, c.lastResetAt
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
