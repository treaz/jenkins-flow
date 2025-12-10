package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/treaz/jenkins-flow/pkg/logger"
)

const defaultPollInterval = 30 * time.Second

// Client handles interaction with the GitHub API
type Client struct {
	Token      string
	HTTPClient *http.Client
	Logger     *logger.Logger
}

// NewClient creates a new GitHub API client
func NewClient(token string, l *logger.Logger) *Client {
	return &Client{
		Token:  token,
		Logger: l,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &logger.LoggingRoundTripper{
				Wrapped: http.DefaultTransport,
				Logger:  l,
			},
		},
	}
}

// PRStatus represents the state of a Pull Request
type PRStatus struct {
	Number   int        `json:"number"`
	State    string     `json:"state"` // "open", "closed"
	Merged   bool       `json:"merged"`
	MergedAt *time.Time `json:"merged_at,omitempty"`
	Title    string     `json:"title"`
	HTMLURL  string     `json:"html_url"`
}

// GetPRStatus fetches the current status of a Pull Request
func (c *Client) GetPRStatus(ctx context.Context, owner, repo string, prNumber int) (*PRStatus, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("PR #%d not found in %s/%s", prNumber, owner, repo)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var pr PRStatus
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub response: %w", err)
	}

	return &pr, nil
}

// WaitForPRStatus polls until the PR reaches the target state.
// Supported target states: "merged", "closed"
func (c *Client) WaitForPRStatus(ctx context.Context, owner, repo string, prNumber int, targetState string, pollInterval time.Duration) error {
	if pollInterval == 0 {
		pollInterval = defaultPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Check immediately first
	if done, err := c.checkPRState(ctx, owner, repo, prNumber, targetState); err != nil {
		return err
	} else if done {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			done, err := c.checkPRState(ctx, owner, repo, prNumber, targetState)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			c.Logger.Debugf("  -> PR #%d: still waiting for state %q...", prNumber, targetState)
		}
	}
}

// checkPRState checks if PR has reached target state
func (c *Client) checkPRState(ctx context.Context, owner, repo string, prNumber int, targetState string) (bool, error) {
	pr, err := c.GetPRStatus(ctx, owner, repo, prNumber)
	if err != nil {
		return false, err
	}

	switch targetState {
	case "merged":
		if pr.Merged {
			c.Logger.Infof("  -> PR #%d is merged!", prNumber)
			return true, nil
		}
		// If PR is closed but not merged, it won't become merged
		if pr.State == "closed" && !pr.Merged {
			return false, fmt.Errorf("PR #%d was closed without being merged", prNumber)
		}
	case "closed":
		if pr.State == "closed" {
			c.Logger.Infof("  -> PR #%d is closed (merged: %v)", prNumber, pr.Merged)
			return true, nil
		}
	default:
		return false, fmt.Errorf("unsupported target state: %q (use 'merged' or 'closed')", targetState)
	}

	return false, nil
}
