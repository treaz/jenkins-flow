package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleListWorkflows(t *testing.T) {
	// Create temporary workflows directory
	tmpDir, err := os.MkdirTemp("", "workflows_test_")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create valid workflow file
	validContent := "name: \"Valid Workflow\"\nworkflow:\n  - name: step1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "valid.yaml"), []byte(validContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create invalid workflow file (missing name)
	invalidContent := "workflow:\n  - name: step1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte(invalidContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-yaml file
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize server
	srv := NewServer(8080, "instances.yaml", tmpDir)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/workflows", nil)
	w := httptest.NewRecorder()

	// Call handler
	srv.handleListWorkflows(w, req)

	// Verify response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}

	var workflows []WorkflowInfo
	if err := json.NewDecoder(resp.Body).Decode(&workflows); err != nil {
		t.Fatal(err)
	}

	// Should contain exactly 1 valid workflow
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}

	if workflows[0].Name != "Valid Workflow" {
		t.Errorf("expected workflow name 'Valid Workflow', got %q", workflows[0].Name)
	}

	expectedPath := filepath.Join(tmpDir, "valid.yaml")
	if workflows[0].Path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, workflows[0].Path)
	}
}
