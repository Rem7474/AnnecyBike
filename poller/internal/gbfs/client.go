package gbfs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func fetch[T any](ctx context.Context, c *Client, endpoint string) (*Feed[T], error) {
	url := c.baseURL + "/" + endpoint
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
		resp, err := c.httpClient.Do(req)
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
			return nil, fmt.Errorf("decode %s: %w", endpoint, err)
		}
		return &result, nil
	}
	return nil, fmt.Errorf("all retries failed for %s: %w", endpoint, lastErr)
}

func (c *Client) VehicleTypes(ctx context.Context) (*Feed[VehicleTypesData], error) {
	return fetch[VehicleTypesData](ctx, c, "vehicle_types.json")
}

func (c *Client) StationInfo(ctx context.Context) (*Feed[StationInfoData], error) {
	return fetch[StationInfoData](ctx, c, "station_information.json")
}

func (c *Client) StationStatus(ctx context.Context) (*Feed[StationStatusData], error) {
	return fetch[StationStatusData](ctx, c, "station_status.json")
}

func (c *Client) FreeBikeStatus(ctx context.Context) (*Feed[FreeBikeData], error) {
	return fetch[FreeBikeData](ctx, c, "free_bike_status.json")
}
