package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/logger"
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
				"number":   1,
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

	l := logger.New(logger.Error)
	result, buildNumber, _, err := runStep(context.Background(), cfg, step, l, nil, 0, 0, NewOutputs())
	if err != nil {
		t.Fatalf("runStep failed: %v", err)
	}

	if result != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %q", result)
	}
	if buildNumber != 1 {
		t.Errorf("expected build number 1, got %d", buildNumber)
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

	l := logger.New(logger.Error)
	results, err := runParallelGroup(context.Background(), cfg, steps, l, NewOutputs())
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
				"number":   1,
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

	l := logger.New(logger.Error)
	_, err := runParallelGroup(context.Background(), cfg, steps, l, NewOutputs())
	if err == nil {
		t.Fatal("expected error from runParallelGroup, got nil")
	}
}

// mockBuildAndDeployServer simulates a build job that returns build number 7777,
// then captures parameters sent to a downstream deploy job. The captured params are
// returned via the supplied map.
func mockBuildAndDeployServer(t *testing.T, deployParams *sync.Map) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/job/build/buildWithParameters" || r.URL.Path == "/job/build/build":
			w.Header().Set("Location", server.URL+"/queue/item/100/")
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/queue/item/100/api/json":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"executable": map[string]string{"url": server.URL + "/job/build/7777/"},
			})
		case r.URL.Path == "/job/build/7777/api/json":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"building": false,
				"result":   "SUCCESS",
				"number":   7777,
			})

		case r.URL.Path == "/job/deploy/buildWithParameters" || r.URL.Path == "/job/deploy/build":
			// Capture every param the deploy job was triggered with.
			if err := r.ParseForm(); err == nil {
				for k, vs := range r.URL.Query() {
					if len(vs) > 0 {
						deployParams.Store(k, vs[0])
					}
				}
			}
			w.Header().Set("Location", server.URL+"/queue/item/200/")
			w.WriteHeader(http.StatusCreated)
		case r.URL.Path == "/queue/item/200/api/json":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"executable": map[string]string{"url": server.URL + "/job/deploy/1/"},
			})
		case r.URL.Path == "/job/deploy/1/api/json":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"building": false,
				"result":   "SUCCESS",
				"number":   1,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	return server
}

func TestRunWithCallbacks_StepOutputSubstitution(t *testing.T) {
	var deployParams sync.Map
	server := mockBuildAndDeployServer(t, &deployParams)
	defer server.Close()

	cfg := &config.Config{
		Instances: map[string]config.Instance{
			"test": {URL: server.URL, Token: "user:token"},
		},
		Workflow: []config.WorkflowItem{
			{
				Name:     "Build NOS Docker Image",
				ID:       "build_nos",
				Instance: "test",
				Job:      "/job/build",
			},
			{
				Parallel: &config.ParallelGroup{
					Name: "Deploy",
					Steps: []config.Step{
						{
							Name:     "Deploy NOS US",
							Instance: "test",
							Job:      "/job/deploy",
							Params: map[string]string{
								"tag": "${steps.build_nos.build_number}",
							},
						},
					},
				},
			},
		},
	}

	l := logger.New(logger.Error)
	if err := RunWithCallbacks(context.Background(), cfg, l, nil, DisabledSet{}); err != nil {
		t.Fatalf("RunWithCallbacks failed: %v", err)
	}

	got, ok := deployParams.Load("tag")
	if !ok {
		t.Fatal("deploy job was not triggered with a 'tag' parameter")
	}
	if got != "7777" {
		t.Errorf("expected tag=7777 (upstream build number), got %q", got)
	}
}

func TestRunWithCallbacks_MixedWorkflow(t *testing.T) {
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

	l := logger.New(logger.Error)
	err := RunWithCallbacks(context.Background(), cfg, l, nil, DisabledSet{})
	if err != nil {
		t.Fatalf("RunWithCallbacks failed: %v", err)
	}

	// Total: 1 + 2 + 1 = 4 triggers
	if triggered != 4 {
		t.Errorf("expected 4 triggers, got %d", triggered)
	}
}
