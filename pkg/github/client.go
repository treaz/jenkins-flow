package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	Number         int        `json:"number"`
	State          string     `json:"state"` // "open", "closed"
	Merged         bool       `json:"merged"`
	MergedAt       *time.Time `json:"merged_at,omitempty"`
	Title          string     `json:"title"`
	HTMLURL        string     `json:"html_url"`
	MergeableState string     `json:"mergeable_state"` // "clean", "behind", "blocked", "dirty", "unstable", "unknown"
	Head           struct {
		Ref string `json:"ref"`
	} `json:"head"`
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

// FindPRByBranch locates an open PR targeting the specified branch. Matching is case-insensitive.
// Returns an error when no PRs or multiple PRs exist for the branch.
func (c *Client) FindPRByBranch(ctx context.Context, owner, repo, branch string) (*PRStatus, error) {
	if branch == "" {
		return nil, fmt.Errorf("branch name must be provided")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&per_page=100", owner, repo)

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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API error (status %d): %s", resp.StatusCode, string(body))
	}

	var pulls []PRStatus
	if err := json.NewDecoder(resp.Body).Decode(&pulls); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub response: %w", err)
	}

	var matches []*PRStatus
	for i := range pulls {
		if strings.EqualFold(pulls[i].Head.Ref, branch) {
			matches = append(matches, &pulls[i])
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no open PR found for branch %q", branch)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("multiple open PRs found for branch %q", branch)
	}
}

// UpdateBranch triggers a server-side merge of the PR's base branch into its head branch.
// Uses GitHub's default merge strategy (no rebase). The endpoint returns 202 Accepted on success.
// 422 is treated as a no-op (head already up to date).
func (c *Client) UpdateBranch(ctx context.Context, owner, repo string, prNumber int) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/update-branch", owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader("{}"))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("update-branch request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusUnprocessableEntity:
		// 422: branch already up to date — treat as no-op
		return nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update-branch failed (status %d): %s", resp.StatusCode, string(body))
	}
}

// WaitForPRStatus polls until the PR reaches the target state and returns the final PR status.
// Supported target states: "merged", "closed".
// When autoUpdateBranch is true and target is "merged", the head branch is auto-updated
// from the base whenever the PR is detected as "behind". An update failure aborts the wait.
func (c *Client) WaitForPRStatus(ctx context.Context, owner, repo string, prNumber int, targetState string, pollInterval time.Duration, autoUpdateBranch bool) (*PRStatus, error) {
	if pollInterval == 0 {
		pollInterval = defaultPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Check immediately first
	if done, pr, err := c.checkPRState(ctx, owner, repo, prNumber, targetState, autoUpdateBranch); err != nil {
		return nil, err
	} else if done {
		return pr, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			done, pr, err := c.checkPRState(ctx, owner, repo, prNumber, targetState, autoUpdateBranch)
			if err != nil {
				return nil, err
			}
			if done {
				return pr, nil
			}
			c.Logger.Debugf("  -> PR #%d: still waiting for state %q...", prNumber, targetState)
		}
	}
}

// checkPRState checks if PR has reached target state.
// If autoUpdateBranch is true and the PR is behind base, triggers update-branch first.
func (c *Client) checkPRState(ctx context.Context, owner, repo string, prNumber int, targetState string, autoUpdateBranch bool) (bool, *PRStatus, error) {
	pr, err := c.GetPRStatus(ctx, owner, repo, prNumber)
	if err != nil {
		return false, nil, err
	}

	if autoUpdateBranch && targetState == "merged" && pr.State == "open" && pr.MergeableState == "behind" {
		c.Logger.Infof("  -> PR #%d is behind base, auto-updating branch...", prNumber)
		if err := c.UpdateBranch(ctx, owner, repo, prNumber); err != nil {
			return false, pr, fmt.Errorf("auto-update of PR #%d branch failed: %w", prNumber, err)
		}
		c.Logger.Infof("  -> PR #%d branch update accepted", prNumber)
	}

	switch targetState {
	case "merged":
		if pr.Merged {
			c.Logger.Infof("  -> PR #%d is merged!", prNumber)
			return true, pr, nil
		}
		// If PR is closed but not merged, it won't become merged
		if pr.State == "closed" && !pr.Merged {
			return false, pr, fmt.Errorf("PR #%d was closed without being merged", prNumber)
		}
	case "closed":
		if pr.State == "closed" {
			c.Logger.Infof("  -> PR #%d is closed (merged: %v)", prNumber, pr.Merged)
			return true, pr, nil
		}
	default:
		return false, pr, fmt.Errorf("unsupported target state: %q (use 'merged' or 'closed')", targetState)
	}

	return false, pr, nil
}
