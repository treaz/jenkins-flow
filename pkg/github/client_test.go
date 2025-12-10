package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/treaz/jenkins-flow/pkg/logger"
)

type rewriteTransport struct {
	base *url.URL
	rt   http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.base.Scheme
	cloned.URL.Host = t.base.Host
	return t.rt.RoundTrip(cloned)
}

func newTestClient(serverURL string) *Client {
	l := logger.New(logger.Error)
	client := NewClient("", l)

	parsed, _ := url.Parse(serverURL)
	client.HTTPClient = &http.Client{
		Transport: &logger.LoggingRoundTripper{
			Wrapped: &rewriteTransport{base: parsed, rt: http.DefaultTransport},
			Logger:  l,
		},
	}

	return client
}

func TestFindPRByBranch_SingleMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/org/repo/pulls" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"number": 42, "head": {"ref": "release/v1"}, "html_url": "https://example.com/pr/42"}
		]`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	pr, err := client.FindPRByBranch(context.Background(), "org", "repo", "Release/V1")
	if err != nil {
		t.Fatalf("FindPRByBranch returned error: %v", err)
	}
	if pr.Number != 42 {
		t.Fatalf("expected PR number 42, got %d", pr.Number)
	}
}

func TestFindPRByBranch_MultipleMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[
			{"number": 1, "head": {"ref": "release/v1"}, "html_url": "https://example.com/pr/1"},
			{"number": 2, "head": {"ref": "RELEASE/V1"}, "html_url": "https://example.com/pr/2"}
		]`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := client.FindPRByBranch(context.Background(), "org", "repo", "release/v1")
	if err == nil || !strings.Contains(err.Error(), "multiple open PRs") {
		t.Fatalf("expected multiple PRs error, got %v", err)
	}
}

func TestFindPRByBranch_NoMatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	_, err := client.FindPRByBranch(context.Background(), "org", "repo", "release/v1")
	if err == nil || !strings.Contains(err.Error(), "no open PR") {
		t.Fatalf("expected no PR error, got %v", err)
	}
}
