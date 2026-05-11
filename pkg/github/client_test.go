package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

func TestUpdateBranch_Accepted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/repos/org/repo/pulls/7/update-branch" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	if err := client.UpdateBranch(context.Background(), "org", "repo", 7); err != nil {
		t.Fatalf("UpdateBranch returned error: %v", err)
	}
}

func TestUpdateBranch_AlreadyUpToDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	if err := client.UpdateBranch(context.Background(), "org", "repo", 7); err != nil {
		t.Fatalf("422 should be tolerated, got error: %v", err)
	}
}

func TestUpdateBranch_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.UpdateBranch(context.Background(), "org", "repo", 7)
	if err == nil || !strings.Contains(err.Error(), "status 403") {
		t.Fatalf("expected 403 error, got %v", err)
	}
}

func TestWaitForPRStatus_AutoUpdateBehindThenMerged(t *testing.T) {
	var getCalls int32
	var updateCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/org/repo/pulls/9":
			n := atomic.AddInt32(&getCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			if n == 1 {
				w.Write([]byte(`{"number":9,"state":"open","merged":false,"mergeable_state":"behind","title":"t","html_url":"https://example.com/pr/9"}`))
			} else {
				w.Write([]byte(`{"number":9,"state":"closed","merged":true,"mergeable_state":"clean","title":"t","html_url":"https://example.com/pr/9"}`))
			}
		case r.Method == http.MethodPut && r.URL.Path == "/repos/org/repo/pulls/9/update-branch":
			atomic.AddInt32(&updateCalls, 1)
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)

	pr, err := client.WaitForPRStatus(context.Background(), "org", "repo", 9, "merged", 10*time.Millisecond, true)
	if err != nil {
		t.Fatalf("WaitForPRStatus returned error: %v", err)
	}
	if !pr.Merged {
		t.Fatalf("expected merged PR, got %+v", pr)
	}
	if got := atomic.LoadInt32(&updateCalls); got != 1 {
		t.Fatalf("expected exactly 1 update-branch call, got %d", got)
	}
	if got := atomic.LoadInt32(&getCalls); got < 2 {
		t.Fatalf("expected at least 2 GET calls, got %d", got)
	}
}

func TestWaitForPRStatus_AutoUpdateDisabled(t *testing.T) {
	var updateCalls int32
	mergedAfter := int32(2)
	var getCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/org/repo/pulls/9":
			n := atomic.AddInt32(&getCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			if n < mergedAfter {
				w.Write([]byte(`{"number":9,"state":"open","merged":false,"mergeable_state":"behind"}`))
			} else {
				w.Write([]byte(`{"number":9,"state":"closed","merged":true,"mergeable_state":"clean"}`))
			}
		case r.Method == http.MethodPut:
			atomic.AddInt32(&updateCalls, 1)
			t.Fatalf("update-branch should not be called when autoUpdate=false")
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	if _, err := client.WaitForPRStatus(context.Background(), "org", "repo", 9, "merged", 10*time.Millisecond, false); err != nil {
		t.Fatalf("WaitForPRStatus returned error: %v", err)
	}
	if got := atomic.LoadInt32(&updateCalls); got != 0 {
		t.Fatalf("update-branch must not be called, got %d", got)
	}
}

func TestWaitForPRStatus_AutoUpdateFailureAborts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"number":9,"state":"open","merged":false,"mergeable_state":"behind"}`))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"message":"merge conflict"}`)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.WaitForPRStatus(context.Background(), "org", "repo", 9, "merged", 10*time.Millisecond, true)
	if err == nil || !strings.Contains(err.Error(), "auto-update") {
		t.Fatalf("expected auto-update error, got %v", err)
	}
}
