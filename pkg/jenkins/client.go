package jenkins

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client handles interaction with a single Jenkins instance
type Client struct {
	BaseURL    string
	AuthToken  string // Can be "user:token" or just "token" (for Bearer)
	HTTPClient *http.Client
}

// NewClient creates a newly configured Jenkins client
func NewClient(baseURL, authToken string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		AuthToken: authToken,
		HTTPClient: &http.Client{
			// Moderate timeout for API calls, but not for the polling loops themselves
			Timeout: 30 * time.Second,
		},
	}
}

// Helper to add authentication headers
func (c *Client) addAuth(req *http.Request) {
	if strings.Contains(c.AuthToken, ":") {
		// Basic Auth (User:APIToken)
		auth := base64.StdEncoding.EncodeToString([]byte(c.AuthToken))
		req.Header.Set("Authorization", "Basic "+auth)
	} else {
		// Bearer Token
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}
}

// TriggerJob starts a job and returns the Queue Item URL
// If params is non-empty, uses /buildWithParameters endpoint
func (c *Client) TriggerJob(ctx context.Context, jobPath string, params map[string]string) (string, error) {
	if !strings.HasPrefix(jobPath, "/") {
		jobPath = "/" + jobPath
	}

	// Choose endpoint based on whether we have parameters
	endpoint := "/build"
	if len(params) > 0 {
		endpoint = "/buildWithParameters"
	}
	targetURL := c.BaseURL + jobPath + endpoint

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, nil)
	if err != nil {
		return "", err
	}
	c.addAuth(req)

	// Add parameters as query string
	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("trigger job request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("trigger failed with status %d: %s", resp.StatusCode, string(body))
	}

	queueItemURL := resp.Header.Get("Location")
	if queueItemURL == "" {
		return "", fmt.Errorf("no Location header returned from trigger")
	}

	return queueItemURL, nil
}

// WaitForQueue waits for a queue item to become a build and returns the Build URL
func (c *Client) WaitForQueue(ctx context.Context, queueItemURL string) (string, error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			// Queue Item URL often ends with /, make sure we append api/json correctly
			qURL := queueItemURL
			if !strings.HasSuffix(qURL, "/") {
				qURL += "/"
			}

			req, err := http.NewRequestWithContext(ctx, "GET", qURL+"api/json", nil)
			if err != nil {
				return "", err
			}
			c.addAuth(req)

			resp, err := c.HTTPClient.Do(req)
			if err != nil {
				return "", fmt.Errorf("poll queue request failed: %w", err)
			}

			if resp.StatusCode == http.StatusNotFound {
				resp.Body.Close()
				// If queue item is gone, it's either cancelled or already processed and we missed the transitions (unlikely with polling).
				// Or Jenkins cleanup removed it.
				return "", fmt.Errorf("queue item not found (cancelled?)")
			}

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return "", fmt.Errorf("poll queue status %d: %s", resp.StatusCode, string(body))
			}

			var result struct {
				Executable struct {
					URL string `json:"url"`
				} `json:"executable"`
				Cancelled bool `json:"cancelled"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				return "", fmt.Errorf("failed to decode queue json: %w", err)
			}
			resp.Body.Close()

			if result.Cancelled {
				return "", fmt.Errorf("job was cancelled in queue")
			}

			if result.Executable.URL != "" {
				return result.Executable.URL, nil
			}
			// Still waiting in queue...
		}
	}
}

// WaitForBuild waits for the build to complete and returns the Result (e.g., SUCCESS, FAILURE)
func (c *Client) WaitForBuild(ctx context.Context, buildURL string) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	if !strings.HasSuffix(buildURL, "/") {
		buildURL += "/"
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", buildURL+"api/json", nil)
			if err != nil {
				return "", err
			}
			c.addAuth(req)

			resp, err := c.HTTPClient.Do(req)
			if err != nil {
				return "", fmt.Errorf("poll build request failed: %w", err)
			}

			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return "", fmt.Errorf("poll build status %d: %s", resp.StatusCode, string(body))
			}

			var result struct {
				Building bool   `json:"building"`
				Result   string `json:"result"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				return "", fmt.Errorf("failed to decode build json: %w", err)
			}
			resp.Body.Close()

			if !result.Building {
				return result.Result, nil
			}
			// Still building...
		}
	}
}
