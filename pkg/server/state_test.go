package server

import (
	"testing"
)

func TestUpdateStepStatusBuildURLPersistence(t *testing.T) {
	sm := NewStateManager()

	items := []WorkflowItemState{
		{
			Step: &StepState{
				Name:     "Build",
				Instance: "ci",
				Job:      "/job/build",
				Status:   StatusPending,
				BuildURL: "https://old.example.com",
			},
		},
	}

	sm.StartWorkflow("test", nil, items)

	sm.UpdateStepStatus(0, 0, StatusRunning, "", "", "")
	if got := sm.GetState().Items[0].Step.BuildURL; got != "" {
		t.Fatalf("expected build URL to be cleared, got %q", got)
	}

	const buildURL = "https://jenkins.example.com/job/12345"
	sm.UpdateStepStatus(0, 0, StatusRunning, "", "", buildURL)
	if got := sm.GetState().Items[0].Step.BuildURL; got != buildURL {
		t.Fatalf("expected build URL %q, got %q", buildURL, got)
	}

	sm.UpdateStepStatus(0, 0, StatusSuccess, "SUCCESS", "", "")
	if got := sm.GetState().Items[0].Step.BuildURL; got != buildURL {
		t.Fatalf("expected build URL to persist after completion, got %q", got)
	}
}

func TestPRWaitErrorHandling(t *testing.T) {
	sm := NewStateManager()

	// Create a PR wait item
	items := []WorkflowItemState{
		{
			IsPRWait: true,
			PRWait: &PRWaitState{
				Name:       "Wait for PR",
				Owner:      "test-owner",
				Repo:       "test-repo",
				HeadBranch: "feature-branch",
				WaitFor:    "merged",
				Status:     StatusPending,
			},
		},
	}

	sm.StartWorkflow("test-workflow", nil, items)

	// Start the PR wait
	sm.StartPRWait(0, "Wait for PR", "test-owner", "test-repo", "feature-branch", "merged", 123, "https://github.com/test-owner/test-repo/pull/123", "Test PR Title")

	// Verify PR wait was started
	state := sm.GetState()
	if state.Items[0].PRWait.Status != StatusRunning {
		t.Fatalf("expected PR wait status to be running, got %s", state.Items[0].PRWait.Status)
	}
	if state.Items[0].PRWait.PRNumber != 123 {
		t.Fatalf("expected PR number 123, got %d", state.Items[0].PRWait.PRNumber)
	}
	if state.Items[0].PRWait.HTMLURL != "https://github.com/test-owner/test-repo/pull/123" {
		t.Fatalf("expected HTML URL, got %s", state.Items[0].PRWait.HTMLURL)
	}

	// Fail the PR wait with an error message
	const errorMsg = "PR was closed without being merged"
	sm.FailPRWait(0, errorMsg)

	// Verify error was captured
	state = sm.GetState()
	prWait := state.Items[0].PRWait

	if prWait.Status != StatusFailed {
		t.Fatalf("expected PR wait status to be failed, got %s", prWait.Status)
	}

	if prWait.Error != errorMsg {
		t.Fatalf("expected error message %q, got %q", errorMsg, prWait.Error)
	}

	// Verify timestamps are set
	if prWait.StartedAt == nil {
		t.Fatal("expected StartedAt to be set")
	}
	if prWait.EndedAt == nil {
		t.Fatal("expected EndedAt to be set")
	}
	if prWait.EndedAt.Before(*prWait.StartedAt) {
		t.Fatal("EndedAt should not be before StartedAt")
	}

	// Verify PR metadata is preserved
	if prWait.PRNumber != 123 {
		t.Fatalf("expected PR number to be preserved, got %d", prWait.PRNumber)
	}
	if prWait.HTMLURL != "https://github.com/test-owner/test-repo/pull/123" {
		t.Fatalf("expected HTML URL to be preserved, got %s", prWait.HTMLURL)
	}
	if prWait.Title != "Test PR Title" {
		t.Fatalf("expected title to be preserved, got %s", prWait.Title)
	}
}

func TestPRWaitSuccessHandling(t *testing.T) {
	sm := NewStateManager()

	// Create a PR wait item
	items := []WorkflowItemState{
		{
			IsPRWait: true,
			PRWait: &PRWaitState{
				Name:       "Wait for PR merge",
				Owner:      "test-owner",
				Repo:       "test-repo",
				HeadBranch: "feature-branch",
				WaitFor:    "merged",
				Status:     StatusPending,
			},
		},
	}

	sm.StartWorkflow("test-workflow", nil, items)

	// Start the PR wait
	sm.StartPRWait(0, "Wait for PR merge", "test-owner", "test-repo", "feature-branch", "merged", 456, "https://github.com/test-owner/test-repo/pull/456", "Feature PR")

	// Complete the PR wait successfully
	sm.CompletePRWait(0)

	// Verify success state
	state := sm.GetState()
	prWait := state.Items[0].PRWait

	if prWait.Status != StatusSuccess {
		t.Fatalf("expected PR wait status to be success, got %s", prWait.Status)
	}

	if prWait.Error != "" {
		t.Fatalf("expected no error message on success, got %q", prWait.Error)
	}

	// Verify timestamps are set
	if prWait.StartedAt == nil {
		t.Fatal("expected StartedAt to be set")
	}
	if prWait.EndedAt == nil {
		t.Fatal("expected EndedAt to be set")
	}
}

func TestStepErrorHandling(t *testing.T) {
	sm := NewStateManager()

	// Create a step item
	items := []WorkflowItemState{
		{
			Step: &StepState{
				Name:     "Deploy",
				Instance: "prod",
				Job:      "/job/deploy",
				Status:   StatusPending,
			},
		},
	}

	sm.StartWorkflow("test-workflow", nil, items)

	// Start the step
	sm.UpdateStepStatus(0, 0, StatusRunning, "", "", "https://jenkins.example.com/job/123")

	// Fail the step with an error
	const errorMsg = "build failed with exit code 1"
	sm.UpdateStepStatus(0, 0, StatusFailed, "FAILURE", errorMsg, "https://jenkins.example.com/job/123")

	// Verify error was captured
	state := sm.GetState()
	step := state.Items[0].Step

	if step.Status != StatusFailed {
		t.Fatalf("expected step status to be failed, got %s", step.Status)
	}

	if step.Error != errorMsg {
		t.Fatalf("expected error message %q, got %q", errorMsg, step.Error)
	}

	if step.Result != "FAILURE" {
		t.Fatalf("expected result FAILURE, got %s", step.Result)
	}

	if step.BuildURL != "https://jenkins.example.com/job/123" {
		t.Fatalf("expected build URL to be preserved, got %s", step.BuildURL)
	}
}
