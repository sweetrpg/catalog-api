// Package assets is a minimal client for assets-web's authenticated asset store
// (GET/POST/DELETE /asset/<kind>/<id>), used to promote a staged cover/sample to live (or
// reclaim it) on volume edit session finalize/accept/reject. See durable-volume-editing in
// sweetrpg/platform.
package assets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Client calls assets-web's asset store endpoints.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a Client against assets-web's base URL. An empty baseURL is accepted so the
// service can still start when ASSETS_WEB_URL isn't configured; every call will then fail with
// a transport error.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http: &http.Client{
			Timeout:   10 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

// NotFoundError means assets-web has no asset at the given kind/id.
type NotFoundError struct{ Kind, ID string }

func (e NotFoundError) Error() string { return fmt.Sprintf("assets: %s/%s not found", e.Kind, e.ID) }

// Get downloads an asset's bytes and content type.
func (c *Client) Get(ctx context.Context, token, kind, id string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/asset/"+kind+"/"+id, nil)
	if err != nil {
		return nil, "", fmt.Errorf("assets: build get request: %w", err)
	}
	setBearerToken(req, token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("assets: get request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", NotFoundError{Kind: kind, ID: id}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("assets: unexpected status %d from get %s/%s", resp.StatusCode, kind, id)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("assets: read get response: %w", err)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// Store uploads an asset's bytes under kind/id, overwriting any existing asset there.
func (c *Client) Store(ctx context.Context, token, kind, id string, data []byte, contentType string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, id))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("assets: build multipart form: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("assets: write multipart body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("assets: close multipart form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/asset/"+kind+"/"+id, &buf)
	if err != nil {
		return fmt.Errorf("assets: build store request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	setBearerToken(req, token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("assets: store request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("assets: unexpected status %d from store %s/%s", resp.StatusCode, kind, id)
	}
	return nil
}

// Delete removes an asset. A missing asset (404) is treated as success - the end state (no
// asset at kind/id) is the same either way.
func (c *Client) Delete(ctx context.Context, token, kind, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/asset/"+kind+"/"+id, nil)
	if err != nil {
		return fmt.Errorf("assets: build delete request: %w", err)
	}
	setBearerToken(req, token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("assets: delete request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("assets: unexpected status %d from delete %s/%s", resp.StatusCode, kind, id)
	}
	return nil
}

// Promote copies a staged asset (fromKind/fromID) to a live one (toKind/toID), then deletes the
// staged copy - the promote-then-delete step every editor/admin finalize and accepted proposal
// performs for a referenced staged cover/sample.
func (c *Client) Promote(ctx context.Context, token, fromKind, fromID, toKind, toID string) error {
	data, contentType, err := c.Get(ctx, token, fromKind, fromID)
	if err != nil {
		return fmt.Errorf("assets: promote %s/%s -> %s/%s: %w", fromKind, fromID, toKind, toID, err)
	}
	if err := c.Store(ctx, token, toKind, toID, data, contentType); err != nil {
		return fmt.Errorf("assets: promote %s/%s -> %s/%s: %w", fromKind, fromID, toKind, toID, err)
	}
	return c.Delete(ctx, token, fromKind, fromID)
}

// setBearerToken forwards the caller's verified user token to assets-web, which authorizes the
// write against the same user rather than trusting this service's identity. See platform's
// api-client-auth change (openspec/changes/api-client-auth).
func setBearerToken(req *http.Request, token string) {
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}
