package osrs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client handles API communication with RuneScape Wiki API
type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

// NewClient creates a new OSRS API client
// userAgent is required by the RuneScape Wiki API
func NewClient(userAgent string) *Client {
	return &Client{
		baseURL:    "https://prices.runescape.wiki/api/v1/osrs",
		userAgent:  userAgent,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// makeAPIRequest is the core HTTP request method (equivalent to make_api_request in Python)
func (c *Client) makeAPIRequest(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Critical: User-Agent required by RuneScape Wiki API
	req.Header.Set("User-Agent", c.userAgent)

	// Add query parameters
	if params != nil {
		q := req.URL.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return body, nil
}

// GetLatestPrices fetches current prices for all items or a specific item
// Equivalent to get_latest_prices method in Python
func (c *Client) GetLatestPrices(ctx context.Context, itemID *int) (*LatestPricesResponse, error) {
	endpoint := "/latest"
	var params map[string]string

	if itemID != nil {
		params = map[string]string{"id": strconv.Itoa(*itemID)}
	}

	data, err := c.makeAPIRequest(ctx, endpoint, params)
	if err != nil {
		return nil, fmt.Errorf("fetching latest prices: %w", err)
	}

	var response LatestPricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("parsing latest prices response: %w", err)
	}

	return &response, nil
}

// GetItemMapping fetches item metadata (names, buy limits, etc.)
// Equivalent to get_item_mapping method in Python
func (c *Client) GetItemMapping(ctx context.Context) ([]ItemMapping, error) {
	data, err := c.makeAPIRequest(ctx, "/mapping", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching item mapping: %w", err)
	}

	var mappings []ItemMapping
	if err := json.Unmarshal(data, &mappings); err != nil {
		return nil, fmt.Errorf("parsing item mapping response: %w", err)
	}

	return mappings, nil
}

// GetTimeseries fetches historical price/volume data for a specific item
// Equivalent to get_timeseries method in Python
func (c *Client) GetTimeseries(ctx context.Context, itemID int, timestep string) (map[string]interface{}, error) {
	params := map[string]string{
		"id":       strconv.Itoa(itemID),
		"timestep": timestep,
	}

	data, err := c.makeAPIRequest(ctx, "/timeseries", params)
	if err != nil {
		return nil, fmt.Errorf("fetching timeseries for item %d: %w", itemID, err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing timeseries response: %w", err)
	}

	return result, nil
}
