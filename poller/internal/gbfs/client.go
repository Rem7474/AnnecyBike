package gbfs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client fetches GBFS feeds using URLs discovered from the provider's gbfs.json.
// All feed URLs are resolved at construction time via auto-discovery, so changing
// the single discovery URL (GBFS_URL env var) is sufficient to switch providers.
type Client struct {
	httpClient *http.Client
	feedURLs   map[string]string // feed name → discovered URL
}

// NewClient fetches the GBFS discovery feed at discoveryURL, parses all feed URLs,
// and returns a ready-to-use client. Returns an error if discovery fails.
func NewClient(ctx context.Context, discoveryURL string) (*Client, error) {
	c := &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		feedURLs:   make(map[string]string),
	}
	if err := c.discover(ctx, discoveryURL); err != nil {
		return nil, fmt.Errorf("GBFS discovery from %s: %w", discoveryURL, err)
	}
	return c, nil
}

func (c *Client) discover(ctx context.Context, url string) error {
	feed, err := fetchURL[GbfsDiscoveryData](ctx, c.httpClient, url)
	if err != nil {
		return err
	}
	// Prefer French, then English, then any available language.
	var refs []GbfsFeedRef
	for _, lang := range []string{"fr", "en"} {
		if d, ok := feed.Data[lang]; ok {
			refs = d.Feeds
			break
		}
	}
	if refs == nil {
		for _, d := range feed.Data {
			refs = d.Feeds
			break
		}
	}
	if len(refs) == 0 {
		return fmt.Errorf("no feeds found in discovery response")
	}
	for _, f := range refs {
		c.feedURLs[f.Name] = f.URL
	}
	return nil
}

// feedURL returns the discovered URL for the given GBFS feed name.
func (c *Client) feedURL(name string) (string, error) {
	u, ok := c.feedURLs[name]
	if !ok {
		return "", fmt.Errorf("feed %q not found in discovery (available: %v)", name, c.knownFeeds())
	}
	return u, nil
}

func (c *Client) knownFeeds() []string {
	names := make([]string, 0, len(c.feedURLs))
	for n := range c.feedURLs {
		names = append(names, n)
	}
	return names
}

// fetchURL performs an HTTP GET with up to 3 retries and decodes into Feed[T].
func fetchURL[T any](ctx context.Context, httpClient *http.Client, url string) (*Feed[T], error) {
	var lastErr error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
			continue
		}
		var result Feed[T]
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode %s: %w", url, err)
		}
		return &result, nil
	}
	return nil, fmt.Errorf("all retries failed for %s: %w", url, lastErr)
}

func (c *Client) VehicleTypes(ctx context.Context) (*Feed[VehicleTypesData], error) {
	u, err := c.feedURL("vehicle_types")
	if err != nil {
		return nil, err
	}
	return fetchURL[VehicleTypesData](ctx, c.httpClient, u)
}

func (c *Client) StationInfo(ctx context.Context) (*Feed[StationInfoData], error) {
	u, err := c.feedURL("station_information")
	if err != nil {
		return nil, err
	}
	return fetchURL[StationInfoData](ctx, c.httpClient, u)
}

func (c *Client) StationStatus(ctx context.Context) (*Feed[StationStatusData], error) {
	u, err := c.feedURL("station_status")
	if err != nil {
		return nil, err
	}
	return fetchURL[StationStatusData](ctx, c.httpClient, u)
}

func (c *Client) FreeBikeStatus(ctx context.Context) (*Feed[FreeBikeData], error) {
	u, err := c.feedURL("free_bike_status")
	if err != nil {
		return nil, err
	}
	return fetchURL[FreeBikeData](ctx, c.httpClient, u)
}

func (c *Client) GeofencingZones(ctx context.Context) (*Feed[GeofencingZonesData], error) {
	u, err := c.feedURL("geofencing_zones")
	if err != nil {
		return nil, err
	}
	return fetchURL[GeofencingZonesData](ctx, c.httpClient, u)
}
