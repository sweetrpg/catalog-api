// Package sysdeletes reconciles catalog's game system tombstones against game-systems-api's
// list of deleted systems, so a dropped delete event isn't lost (PADR-0047). It sends no
// credential (PADR-0037), and only ever learns deletions from what the list contains: a missing
// entry or a failed request never counts as one.
package sysdeletes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sweetrpg/catalog-api/internal/events"
)

const pageSize = 100

// Client reads GET /deleted-systems.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a client against game-systems-api's base URL.
func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 10 * time.Second}}
}

type page struct {
	Systems []struct {
		ID        string    `json:"id"`
		SystemID  string    `json:"system_id"`
		DeletedAt time.Time `json:"deleted_at"`
	} `json:"systems"`
	Total int `json:"total"`
}

// List returns every deleted system. Any failed page fails the whole call.
func (c *Client) List(ctx context.Context) ([]events.SystemDeletion, error) {
	var all []events.SystemDeletion
	for number := 1; ; number++ {
		got, err := c.page(ctx, number)
		if err != nil {
			return nil, err
		}
		for _, s := range got.Systems {
			if s.ID != "" {
				all = append(all, events.SystemDeletion{ID: s.ID, SystemID: s.SystemID, DeletedAt: s.DeletedAt})
			}
		}
		if len(got.Systems) == 0 || number*pageSize >= got.Total {
			return all, nil
		}
	}
}

func (c *Client) page(ctx context.Context, number int) (*page, error) {
	url := fmt.Sprintf("%s/deleted-systems?page=%d&per_page=%d", c.baseURL, number, pageSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("deleted systems: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deleted systems: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("deleted systems: unexpected status %d", resp.StatusCode)
	}
	var out page
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("deleted systems: decode: %w", err)
	}
	return &out, nil
}
