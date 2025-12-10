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

func TestLoad_ParallelWorkflow(t *testing.T) {
	// 1. Create Instances File
	instancesContent := `
instances:
  us:
    url: http://jenkins-us.example.com
    token: "user:token-us"
  eu:
    url: http://jenkins-eu.example.com
    token: "user:token-eu"
  apac:
    url: http://jenkins-apac.example.com
    token: "user:token-apac"
`
	instancesFile, err := os.CreateTemp("", "instances_parallel_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(instancesFile.Name())
	instancesFile.Write([]byte(instancesContent))
	instancesFile.Close()

	// 2. Create Workflow File with parallel steps
	workflowContent := `
workflow:
  - name: "Build"
    instance: us
    job: "/job/build"
  - parallel:
      name: "Deploy to All Regions"
      steps:
        - name: "Deploy US"
          instance: us
          job: "/job/deploy"
          params:
            REGION: "us-east-1"
        - name: "Deploy EU"
          instance: eu
          job: "/job/deploy"
          params:
            REGION: "eu-west-1"
        - name: "Deploy APAC"
          instance: apac
          job: "/job/deploy"
          params:
            REGION: "ap-southeast-1"
  - name: "Integration Tests"
    instance: us
    job: "/job/integration-tests"
`
	workflowFile, err := os.CreateTemp("", "workflow_parallel_*.yaml")
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

	// Verify workflow structure
	if len(cfg.Workflow) != 3 {
		t.Fatalf("expected 3 workflow items, got %d", len(cfg.Workflow))
	}

	// First item: regular step
	if cfg.Workflow[0].IsParallel() {
		t.Error("first workflow item should not be parallel")
	}
	step := cfg.Workflow[0].AsStep()
	if step.Name != "Build" {
		t.Errorf("expected step name 'Build', got %q", step.Name)
	}

	// Second item: parallel group
	if !cfg.Workflow[1].IsParallel() {
		t.Error("second workflow item should be parallel")
	}
	parallelGroup := cfg.Workflow[1].Parallel
	if parallelGroup.Name != "Deploy to All Regions" {
		t.Errorf("expected parallel group name 'Deploy to All Regions', got %q", parallelGroup.Name)
	}
	if len(parallelGroup.Steps) != 3 {
		t.Fatalf("expected 3 parallel steps, got %d", len(parallelGroup.Steps))
	}
	if parallelGroup.Steps[0].Name != "Deploy US" {
		t.Errorf("expected first parallel step name 'Deploy US', got %q", parallelGroup.Steps[0].Name)
	}
	if parallelGroup.Steps[0].Params["REGION"] != "us-east-1" {
		t.Errorf("expected REGION param 'us-east-1', got %q", parallelGroup.Steps[0].Params["REGION"])
	}

	// Third item: regular step
	if cfg.Workflow[2].IsParallel() {
		t.Error("third workflow item should not be parallel")
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

func TestValidate_EmptyParallelGroup(t *testing.T) {
	// Valid instances
	instancesContent := `
instances:
  local:
    url: "http://localhost"
    token: "user:token"
`
	instancesFile, err := os.CreateTemp("", "instances_empty_parallel_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(instancesFile.Name())
	instancesFile.Write([]byte(instancesContent))
	instancesFile.Close()

	// Workflow with empty parallel group
	workflowContent := `
workflow:
  - parallel:
      name: "Empty Group"
      steps: []
`
	workflowFile, err := os.CreateTemp("", "workflow_empty_parallel_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(workflowFile.Name())
	workflowFile.Write([]byte(workflowContent))
	workflowFile.Close()

	_, err = Load(instancesFile.Name(), workflowFile.Name())
	if err == nil {
		t.Fatal("expected validation error for empty parallel group, got nil")
	}
}

func TestValidate_ParallelStepUnknownInstance(t *testing.T) {
	// Valid instances
	instancesContent := `
instances:
  local:
    url: "http://localhost"
    token: "user:token"
`
	instancesFile, err := os.CreateTemp("", "instances_parallel_unknown_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(instancesFile.Name())
	instancesFile.Write([]byte(instancesContent))
	instancesFile.Close()

	// Workflow with parallel step referencing unknown instance
	workflowContent := `
workflow:
  - parallel:
      steps:
        - name: "Step 1"
          instance: unknown
          job: "/job/test"
`
	workflowFile, err := os.CreateTemp("", "workflow_parallel_unknown_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(workflowFile.Name())
	workflowFile.Write([]byte(workflowContent))
	workflowFile.Close()

	_, err = Load(instancesFile.Name(), workflowFile.Name())
	if err == nil {
		t.Fatal("expected validation error for unknown instance in parallel step, got nil")
	}
}

func TestWorkflowItem_IsParallel(t *testing.T) {
	// Test single step
	item := WorkflowItem{
		Name:     "Test",
		Instance: "local",
		Job:      "/job/test",
	}
	if item.IsParallel() {
		t.Error("expected IsParallel() to return false for single step")
	}

	// Test parallel group
	parallelItem := WorkflowItem{
		Parallel: &ParallelGroup{
			Steps: []Step{
				{Name: "Step 1", Instance: "local", Job: "/job/test"},
			},
		},
	}
	if !parallelItem.IsParallel() {
		t.Error("expected IsParallel() to return true for parallel group")
	}
}

func TestWorkflowItem_AsStep(t *testing.T) {
	item := WorkflowItem{
		Name:     "Test Step",
		Instance: "prod",
		Job:      "/job/deploy",
		Params:   map[string]string{"ENV": "production"},
	}

	step := item.AsStep()
	if step.Name != "Test Step" {
		t.Errorf("expected Name 'Test Step', got %q", step.Name)
	}
	if step.Instance != "prod" {
		t.Errorf("expected Instance 'prod', got %q", step.Instance)
	}
	if step.Job != "/job/deploy" {
		t.Errorf("expected Job '/job/deploy', got %q", step.Job)
	}
	if step.Params["ENV"] != "production" {
		t.Errorf("expected Params['ENV'] 'production', got %q", step.Params["ENV"])
	}
}
