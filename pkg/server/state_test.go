package server

import "testing"

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
