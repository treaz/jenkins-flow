package server

import (
	"sync"
	"time"
)

// StepStatus represents the current status of a workflow step.
type StepStatus string

const (
	StatusPending StepStatus = "pending"
	StatusRunning StepStatus = "running"
	StatusSuccess StepStatus = "success"
	StatusFailed  StepStatus = "failed"
	StatusSkipped StepStatus = "skipped"
)

// StepState holds the state of a single step.
type StepState struct {
	Name      string     `json:"name"`
	Instance  string     `json:"instance"`
	Job       string     `json:"job"`
	Status    StepStatus `json:"status"`
	Result    string     `json:"result,omitempty"`
	Error     string     `json:"error,omitempty"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
	BuildURL  string     `json:"buildUrl,omitempty"`
}

// ParallelGroupState holds the state of a parallel execution group.
type ParallelGroupState struct {
	Name   string      `json:"name"`
	Steps  []StepState `json:"steps"`
	Status StepStatus  `json:"status"`
}

// WorkflowItemState represents either a step or parallel group.
type WorkflowItemState struct {
	IsParallel bool                `json:"isParallel"`
	Step       *StepState          `json:"step,omitempty"`
	Parallel   *ParallelGroupState `json:"parallel,omitempty"`
}

// WorkflowState holds the complete state of a workflow execution.
type WorkflowState struct {
	Name      string              `json:"name"`
	Status    StepStatus          `json:"status"`
	Items     []WorkflowItemState `json:"items"`
	StartedAt *time.Time          `json:"startedAt,omitempty"`
	EndedAt   *time.Time          `json:"endedAt,omitempty"`
	Error     string              `json:"error,omitempty"`
}

// StateManager manages workflow execution state in a thread-safe manner.
type StateManager struct {
	mu      sync.RWMutex
	current *WorkflowState
	running bool
}

// NewStateManager creates a new StateManager.
func NewStateManager() *StateManager {
	return &StateManager{}
}

// IsRunning returns true if a workflow is currently executing.
func (sm *StateManager) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.running
}

// GetState returns a copy of the current workflow state.
func (sm *StateManager) GetState() *WorkflowState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.current == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	state := *sm.current
	return &state
}

// StartWorkflow initializes state for a new workflow execution.
func (sm *StateManager) StartWorkflow(name string, items []WorkflowItemState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	sm.current = &WorkflowState{
		Name:      name,
		Status:    StatusRunning,
		Items:     items,
		StartedAt: &now,
	}
	sm.running = true
}

// UpdateStepStatus updates the status of a specific step.
func (sm *StateManager) UpdateStepStatus(itemIndex int, stepIndex int, status StepStatus, result, errMsg, buildURL string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current == nil || itemIndex >= len(sm.current.Items) {
		return
	}

	item := &sm.current.Items[itemIndex]
	var step *StepState

	if item.IsParallel && item.Parallel != nil {
		if stepIndex >= len(item.Parallel.Steps) {
			return
		}
		step = &item.Parallel.Steps[stepIndex]
	} else if item.Step != nil {
		step = item.Step
	} else {
		return
	}

	now := time.Now()
	step.Status = status
	step.Result = result
	step.Error = errMsg
	step.BuildURL = buildURL

	if status == StatusRunning && step.StartedAt == nil {
		step.StartedAt = &now
	}
	if status == StatusSuccess || status == StatusFailed || status == StatusSkipped {
		step.EndedAt = &now
	}

	// Update parallel group status if applicable
	if item.IsParallel && item.Parallel != nil {
		sm.updateParallelGroupStatus(item.Parallel)
	}
}

// updateParallelGroupStatus updates the overall status of a parallel group.
func (sm *StateManager) updateParallelGroupStatus(pg *ParallelGroupState) {
	allSuccess := true
	anyRunning := false
	anyFailed := false

	for _, step := range pg.Steps {
		switch step.Status {
		case StatusRunning:
			anyRunning = true
			allSuccess = false
		case StatusFailed:
			anyFailed = true
			allSuccess = false
		case StatusPending:
			allSuccess = false
		}
	}

	if anyFailed {
		pg.Status = StatusFailed
	} else if anyRunning {
		pg.Status = StatusRunning
	} else if allSuccess {
		pg.Status = StatusSuccess
	} else {
		pg.Status = StatusPending
	}
}

// CompleteWorkflow marks the workflow as completed.
func (sm *StateManager) CompleteWorkflow(success bool, errMsg string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.current == nil {
		return
	}

	now := time.Now()
	sm.current.EndedAt = &now
	sm.running = false

	if success {
		sm.current.Status = StatusSuccess
	} else {
		sm.current.Status = StatusFailed
		sm.current.Error = errMsg
	}
}

// Reset clears the current state.
func (sm *StateManager) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.current = nil
	sm.running = false
}
