package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// 1. Create Instances File
	instancesContent := `
instances:
  local:
    url: http://localhost:8080
    auth_env: TEST_ENV_VAR
  direct:
    url: http://jenkins.example.com
    token: "user:token"
`
	instancesFile, err := os.CreateTemp("", "instances_test_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(instancesFile.Name())
	instancesFile.Write([]byte(instancesContent))
	instancesFile.Close()

	// 2. Create Workflow File
	workflowContent := `
workflow:
  - name: "Step 1"
    instance: local
    job: "/job/test"
`
	workflowFile, err := os.CreateTemp("", "workflow_test_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(workflowFile.Name())
	workflowFile.Write([]byte(workflowContent))
	workflowFile.Close()

	// Test Load
	cfg, err := Load(instancesFile.Name(), workflowFile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(cfg.Instances))
	}
	if cfg.Instances["local"].URL != "http://localhost:8080" {
		t.Errorf("unexpected URL: %s", cfg.Instances["local"].URL)
	}

	// Test Token Retrieval
	os.Setenv("TEST_ENV_VAR", "env-token")
	token, err := cfg.Instances["local"].GetToken()
	if err != nil {
		t.Errorf("unexpected error getting token for local: %v", err)
	}
	if token != "env-token" {
		t.Errorf("expected 'env-token', got %q", token)
	}
}

func TestValidate_MissingAuth(t *testing.T) {
	// Instances with missing auth
	instancesContent := `
instances:
  bad:
    url: "http://bad"
`
	instancesFile, err := os.CreateTemp("", "instances_bad_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(instancesFile.Name())
	instancesFile.Write([]byte(instancesContent))
	instancesFile.Close()

	// Valid workflow
	workflowContent := `
workflow:
  - name: "Step 1"
    instance: bad
    job: "/job/test"
`
	workflowFile, err := os.CreateTemp("", "workflow_valid_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(workflowFile.Name())
	workflowFile.Write([]byte(workflowContent))
	workflowFile.Close()

	_, err = Load(instancesFile.Name(), workflowFile.Name())
	if err == nil {
		t.Fatal("expected validation error for missing auth, got nil")
	}
}
