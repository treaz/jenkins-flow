package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/treaz/jenkins-flow/pkg/config"
)

// mockJenkinsServer creates a mock Jenkins server that tracks job triggers.
// It returns URLs that point back to itself.
func mockJenkinsServer(triggered *int32) *httptest.Server {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/job/test/build" || r.URL.Path == "/job/test/buildWithParameters":
			// Trigger endpoint
			atomic.AddInt32(triggered, 1)
			w.Header().Set("Location", server.URL+"/queue/item/123/")
			w.WriteHeader(http.StatusCreated)

		case r.URL.Path == "/queue/item/123/api/json":
			// Queue API - return build URL immediately
			json.NewEncoder(w).Encode(map[string]interface{}{
				"executable": map[string]string{"url": server.URL + "/job/test/1/"},
			})

		case r.URL.Path == "/job/test/1/api/json":
			// Build API - return SUCCESS
			json.NewEncoder(w).Encode(map[string]interface{}{
				"building": false,
				"result":   "SUCCESS",
			})

		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func TestRunStep_Success(t *testing.T) {
	var triggered int32
	server := mockJenkinsServer(&triggered)
	defer server.Close()

	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"test": {URL: server.URL, Token: "user:token"},
		},
	}

	step := config.Step{
		Name:     "Test Step",
		Instance: "test",
		Job:      "/job/test",
	}

	result, err := runStep(context.Background(), cfg, step)
	if err != nil {
		t.Fatalf("runStep failed: %v", err)
	}

	if result != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %q", result)
	}

	if triggered != 1 {
		t.Errorf("expected 1 trigger, got %d", triggered)
	}
}

func TestRunParallelGroup_Success(t *testing.T) {
	var triggered int32
	server := mockJenkinsServer(&triggered)
	defer server.Close()

	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"test": {URL: server.URL, Token: "user:token"},
		},
	}

	steps := []config.Step{
		{Name: "Step 1", Instance: "test", Job: "/job/test"},
		{Name: "Step 2", Instance: "test", Job: "/job/test"},
		{Name: "Step 3", Instance: "test", Job: "/job/test"},
	}

	results, err := runParallelGroup(context.Background(), cfg, steps)
	if err != nil {
		t.Fatalf("runParallelGroup failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Error != nil {
			t.Errorf("step %d had error: %v", i, r.Error)
		}
		if r.Result != "SUCCESS" {
			t.Errorf("step %d expected SUCCESS, got %q", i, r.Result)
		}
	}

	// All 3 jobs should have been triggered
	if triggered != 3 {
		t.Errorf("expected 3 triggers, got %d", triggered)
	}
}

// mockFailingJenkinsServer returns FAILURE for job results.
func mockFailingJenkinsServer() *httptest.Server {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/job/test/build" || r.URL.Path == "/job/test/buildWithParameters":
			w.Header().Set("Location", server.URL+"/queue/item/123/")
			w.WriteHeader(http.StatusCreated)

		case r.URL.Path == "/queue/item/123/api/json":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"executable": map[string]string{"url": server.URL + "/job/test/1/"},
			})

		case r.URL.Path == "/job/test/1/api/json":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"building": false,
				"result":   "FAILURE",
			})

		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func TestRunParallelGroup_FailFast(t *testing.T) {
	server := mockFailingJenkinsServer()
	defer server.Close()

	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"test": {URL: server.URL, Token: "user:token"},
		},
	}

	steps := []config.Step{
		{Name: "Step 1", Instance: "test", Job: "/job/test"},
		{Name: "Step 2", Instance: "test", Job: "/job/test"},
	}

	_, err := runParallelGroup(context.Background(), cfg, steps)
	if err == nil {
		t.Fatal("expected error from runParallelGroup, got nil")
	}
}

func TestRun_MixedWorkflow(t *testing.T) {
	var triggered int32
	server := mockJenkinsServer(&triggered)
	defer server.Close()

	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"test": {URL: server.URL, Token: "user:token"},
		},
		Workflow: []config.WorkflowItem{
			// Single step
			{
				Name:     "Build",
				Instance: "test",
				Job:      "/job/test",
			},
			// Parallel group
			{
				Parallel: &config.ParallelGroup{
					Name: "Deploy",
					Steps: []config.Step{
						{Name: "Deploy 1", Instance: "test", Job: "/job/test"},
						{Name: "Deploy 2", Instance: "test", Job: "/job/test"},
					},
				},
			},
			// Another single step
			{
				Name:     "Verify",
				Instance: "test",
				Job:      "/job/test",
			},
		},
	}

	err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Total: 1 + 2 + 1 = 4 triggers
	if triggered != 4 {
		t.Errorf("expected 4 triggers, got %d", triggered)
	}
}
