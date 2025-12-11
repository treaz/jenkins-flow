package config

import (
	"path/filepath"
	"testing"
)

func td(name string) string {
	return filepath.Join("testdata", name)
}

func TestLoad(t *testing.T) {
	cfg, err := Load(td("load_instances.yaml"), td("load_workflow.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(cfg.Instances))
	}
	if cfg.Instances["local"].URL != "http://localhost:8080" {
		t.Errorf("unexpected URL: %s", cfg.Instances["local"].URL)
	}

	t.Setenv("TEST_ENV_VAR", "env-token")
	token, err := cfg.Instances["local"].GetToken()
	if err != nil {
		t.Fatalf("unexpected error getting token for local: %v", err)
	}
	if token != "env-token" {
		t.Errorf("expected 'env-token', got %q", token)
	}
}

func TestLoad_SlackWebhook(t *testing.T) {
	cfg, err := Load(td("slack_instances.yaml"), td("slack_workflow.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := "https://hooks.slack.com/services/T000/B000/XXXX"
	if cfg.SlackWebhook != want {
		t.Fatalf("unexpected Slack webhook: %q", cfg.SlackWebhook)
	}
}

func TestLoad_ParallelWorkflow(t *testing.T) {
	cfg, err := Load(td("parallel_instances.yaml"), td("parallel_workflow.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Workflow) != 3 {
		t.Fatalf("expected 3 workflow items, got %d", len(cfg.Workflow))
	}

	if cfg.Workflow[0].IsParallel() {
		t.Error("first workflow item should not be parallel")
	}
	step := cfg.Workflow[0].AsStep()
	if step.Name != "Build" {
		t.Errorf("expected step name 'Build', got %q", step.Name)
	}

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

	if cfg.Workflow[2].IsParallel() {
		t.Error("third workflow item should not be parallel")
	}
}

func TestValidate_MissingAuth(t *testing.T) {
	_, err := Load(td("missing_auth_instances.yaml"), td("missing_auth_workflow.yaml"))
	if err == nil {
		t.Fatal("expected validation error for missing auth, got nil")
	}
}

func TestValidate_EmptyParallelGroup(t *testing.T) {
	_, err := Load(td("single_local_instance.yaml"), td("empty_parallel_workflow.yaml"))
	if err == nil {
		t.Fatal("expected validation error for empty parallel group, got nil")
	}
}

func TestValidate_ParallelStepUnknownInstance(t *testing.T) {
	_, err := Load(td("single_local_instance.yaml"), td("parallel_unknown_workflow.yaml"))
	if err == nil {
		t.Fatal("expected validation error for unknown instance in parallel step, got nil")
	}
}

func TestWorkflowItem_IsParallel(t *testing.T) {
	item := WorkflowItem{
		Name:     "Test",
		Instance: "local",
		Job:      "/job/test",
	}
	if item.IsParallel() {
		t.Error("expected IsParallel() to return false for single step")
	}

	parallelItem := WorkflowItem{
		Parallel: &ParallelGroup{
			Steps: []Step{{Name: "Step 1", Instance: "local", Job: "/job/test"}},
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

func TestLoad_PRWaitWorkflow(t *testing.T) {
	cfg, err := Load(td("pr_instances.yaml"), td("pr_workflow.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.GitHub == nil {
		t.Fatal("expected GitHub config to be loaded")
	}
	token, err := cfg.GitHub.GetToken()
	if err != nil {
		t.Fatalf("unexpected error getting token: %v", err)
	}
	if token != "gh-token" {
		t.Errorf("expected GitHub token 'gh-token', got %q", token)
	}

	if len(cfg.Workflow) != 2 {
		t.Fatalf("expected 2 workflow items, got %d", len(cfg.Workflow))
	}

	if !cfg.Workflow[0].IsPRWait() {
		t.Error("first workflow item should be PR Wait")
	}
	pr := cfg.Workflow[0].WaitForPR
	if pr.Name != "Wait for Release" {
		t.Errorf("expected PR name 'Wait for Release', got %q", pr.Name)
	}
	if pr.Owner != "treaz" {
		t.Errorf("expected Owner 'treaz', got %q", pr.Owner)
	}
	if pr.PRNumber != 42 {
		t.Errorf("expected PR Number 42, got %d", pr.PRNumber)
	}
	if pr.WaitFor != "merged" {
		t.Errorf("expected WaitFor 'merged', got %q", pr.WaitFor)
	}

	if cfg.Workflow[1].IsPRWait() {
		t.Error("second workflow item should not be PR Wait")
	}
}

func TestLoad_PRWaitWorkflow_HeadBranch(t *testing.T) {
	cfg, err := Load(td("pr_instances.yaml"), td("pr_head_branch_workflow.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !cfg.Workflow[0].IsPRWait() {
		t.Fatal("expected first item to be PR wait")
	}
	pr := cfg.Workflow[0].WaitForPR
	if pr.HeadBranch != "release/v1" {
		t.Fatalf("expected head_branch 'release/v1', got %q", pr.HeadBranch)
	}
	if pr.PRNumber != 0 {
		t.Fatalf("expected pr_number 0, got %d", pr.PRNumber)
	}
}

func TestValidatePRWait_MutuallyExclusiveFields(t *testing.T) {
	_, err := Load(td("pr_instances.yaml"), td("pr_invalid_workflow.yaml"))
	if err == nil {
		t.Fatal("expected validation error when both pr_number and head_branch set")
	}
}

func TestValidatePRWait_MissingIdentifiers(t *testing.T) {
	_, err := Load(td("pr_instances.yaml"), td("pr_missing_workflow.yaml"))
	if err == nil {
		t.Fatal("expected validation error when neither pr_number nor head_branch provided")
	}
}

func TestParseWorkflowMeta(t *testing.T) {
	name, err := ParseWorkflowMeta(td("workflow_meta.yaml"))
	if err != nil {
		t.Fatalf("ParseWorkflowMeta failed: %v", err)
	}
	if name != "My Workflow" {
		t.Errorf("expected name 'My Workflow', got %q", name)
	}

	if _, err := ParseWorkflowMeta(td("workflow_meta_missing_name.yaml")); err == nil {
		t.Error("expected error for missing name, got nil")
	}
}
