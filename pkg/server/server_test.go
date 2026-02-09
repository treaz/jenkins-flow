package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/treaz/jenkins-flow/pkg/api"
	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/logger"
)

func TestHandleListWorkflows(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "workflows_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	workflowsDir := filepath.Join(tmpDir, "workflows")
	if err := os.Mkdir(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create temporary instances file in parent dir (not in workflows dir)
	instancesPath := filepath.Join(tmpDir, "instances.yaml")
	instancesContent := "instances:\n  dev:\n    url: http://localhost:8080\n    token: test:token\n"
	if err := os.WriteFile(instancesPath, []byte(instancesContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create valid workflow file
	validContent := "name: \"Valid Workflow\"\nworkflow:\n  - name: step1\n    instance: dev\n    job: /job/test\n"
	if err := os.WriteFile(filepath.Join(workflowsDir, "valid.yaml"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create invalid workflow file (missing name)
	invalidContent := "workflow:\n  - name: step1\n"
	if err := os.WriteFile(filepath.Join(workflowsDir, "invalid.yaml"), []byte(invalidContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create workflow with unknown instance
	unknownInstanceContent := "name: \"Unknown Instance\"\nworkflow:\n  - name: step1\n    instance: unknown\n    job: /job/test\n"
	if err := os.WriteFile(filepath.Join(workflowsDir, "unknown_instance.yaml"), []byte(unknownInstanceContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-yaml file
	if err := os.WriteFile(filepath.Join(workflowsDir, "readme.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize server
	l := logger.New(logger.Error)
	srv := NewServer(8080, instancesPath, []string{workflowsDir}, "", l)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	w := httptest.NewRecorder()

	// Call handler
	srv.ListWorkflows(w, req)

	// Verify response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}

	var workflows []api.WorkflowInfo
	if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
		t.Fatal(err)
	}

	// Should contain 3 workflows (1 valid, 2 invalid)
	if len(workflows) != 3 {
		t.Logf("Workflows returned:")
		for i, wf := range workflows {
			name := "nil"
			if wf.Name != nil {
				name = *wf.Name
			}
			path := "nil"
			if wf.Path != nil {
				path = *wf.Path
			}
			valid := "nil"
			if wf.Valid != nil {
				if *wf.Valid {
					valid = "true"
				} else {
					valid = "false"
				}
			}
			t.Logf("  [%d] Name=%s, Path=%s, Valid=%s", i, name, path, valid)
		}
		t.Fatalf("expected 3 workflows, got %d", len(workflows))
	}

	// Find each workflow and verify
	var validWF, invalidWF, unknownWF *api.WorkflowInfo
	for i := range workflows {
		wf := &workflows[i]
		if wf.Name != nil {
			switch *wf.Name {
			case "Valid Workflow":
				validWF = wf
			case "invalid.yaml":
				invalidWF = wf
			case "Unknown Instance":
				unknownWF = wf
			}
		}
	}

	// Check valid workflow
	if validWF == nil {
		t.Fatal("valid workflow not found")
	}
	if validWF.Valid == nil || !*validWF.Valid {
		t.Errorf("expected valid workflow to be valid=true, got %v", validWF.Valid)
	}
	if validWF.Error != nil && *validWF.Error != "" {
		t.Errorf("expected no error for valid workflow, got %q", *validWF.Error)
	}

	// Check invalid workflow (missing name)
	if invalidWF == nil {
		t.Fatal("invalid workflow not found")
	}
	if invalidWF.Valid == nil || *invalidWF.Valid {
		t.Errorf("expected invalid workflow to be valid=false, got %v", invalidWF.Valid)
	}
	if invalidWF.Error == nil || *invalidWF.Error == "" {
		t.Error("expected error for invalid workflow")
	}

	// Check workflow with unknown instance
	if unknownWF == nil {
		t.Fatal("unknown instance workflow not found")
	}
	if unknownWF.Valid == nil || *unknownWF.Valid {
		t.Errorf("expected unknown instance workflow to be valid=false, got %v", unknownWF.Valid)
	}
	if unknownWF.Error == nil || *unknownWF.Error == "" {
		t.Error("expected error for unknown instance workflow")
	}
}

func TestApplyInputSubstitutions_PRWaitHeadBranch(t *testing.T) {
	cfg := &config.Config{
		Inputs: map[string]string{
			"git_branch_to_merge": "PAYMENTS-3096_update_threshold",
		},
		Workflow: []config.WorkflowItem{
			{
				WaitForPR: &config.PRWait{
					Name:       "Wait for Release PR",
					Owner:      "chargepoint-emu",
					Repo:       "nos",
					HeadBranch: "${git_branch_to_merge}",
					WaitFor:    "merged",
				},
			},
		},
	}

	srv := &Server{}
	srv.applyInputSubstitutions(cfg)

	got := cfg.Workflow[0].WaitForPR.HeadBranch
	if got != "PAYMENTS-3096_update_threshold" {
		t.Fatalf("expected head_branch to be substituted, got %q", got)
	}
}
