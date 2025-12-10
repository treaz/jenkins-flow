package workflow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/jenkins"
	"golang.org/x/sync/errgroup"
)

// StepResult holds the result of a step execution.
type StepResult struct {
	StepName string
	Result   string
	Error    error
}

// Run executes the defined workflow, supporting both sequential and parallel steps.
func Run(ctx context.Context, cfg *config.Config) error {
	log.Println("Starting workflow execution...")
	start := time.Now()

	for i, item := range cfg.Workflow {
		if item.IsParallel() {
			// Execute parallel group
			groupName := item.Parallel.Name
			if groupName == "" {
				groupName = fmt.Sprintf("Parallel Group %d", i+1)
			}
			log.Printf("[%d/%d] Starting %s (%d steps)...", i+1, len(cfg.Workflow), groupName, len(item.Parallel.Steps))

			results, err := runParallelGroup(ctx, cfg, item.Parallel.Steps)
			if err != nil {
				return fmt.Errorf("parallel group %q failed: %w", groupName, err)
			}

			// Log all results
			for _, r := range results {
				if r.Error != nil {
					log.Printf("  ✗ %s: FAILED - %v", r.StepName, r.Error)
				} else {
					log.Printf("  ✓ %s: %s", r.StepName, r.Result)
				}
			}

			log.Printf("[%d/%d] %s completed successfully.", i+1, len(cfg.Workflow), groupName)
		} else {
			// Execute single step
			step := item.AsStep()
			log.Printf("[Step %d/%d] Starting step %q on instance %q...", i+1, len(cfg.Workflow), step.Name, step.Instance)

			result, err := runStep(ctx, cfg, step)
			if err != nil {
				return fmt.Errorf("step %q failed: %w", step.Name, err)
			}

			log.Printf("  -> Build finished with result: %s", result)
			if result != "SUCCESS" {
				return fmt.Errorf("step %q failed with result: %s", step.Name, result)
			}

			log.Printf("[Step %d/%d] Completed successfully.", i+1, len(cfg.Workflow))
		}
	}

	duration := time.Since(start)
	log.Printf("Workflow completed successfully in %s.", duration)
	return nil
}

// WorkflowCallbacks provides hooks into workflow execution for state tracking.
type WorkflowCallbacks interface {
	OnStepStart(itemIndex, stepIndex int, name, buildURL string)
	OnStepComplete(itemIndex, stepIndex int, name, result string, err error)
}

// RunWithCallbacks executes the workflow with callback notifications.
func RunWithCallbacks(ctx context.Context, cfg *config.Config, callbacks WorkflowCallbacks) error {
	log.Println("Starting workflow execution...")
	start := time.Now()

	for i, item := range cfg.Workflow {
		if item.IsParallel() {
			// Execute parallel group
			groupName := item.Parallel.Name
			if groupName == "" {
				groupName = fmt.Sprintf("Parallel Group %d", i+1)
			}
			log.Printf("[%d/%d] Starting %s (%d steps)...", i+1, len(cfg.Workflow), groupName, len(item.Parallel.Steps))

			results, err := runParallelGroupWithCallbacks(ctx, cfg, item.Parallel.Steps, i, callbacks)
			if err != nil {
				return fmt.Errorf("parallel group %q failed: %w", groupName, err)
			}

			// Log all results
			for _, r := range results {
				if r.Error != nil {
					log.Printf("  ✗ %s: FAILED - %v", r.StepName, r.Error)
				} else {
					log.Printf("  ✓ %s: %s", r.StepName, r.Result)
				}
			}

			log.Printf("[%d/%d] %s completed successfully.", i+1, len(cfg.Workflow), groupName)
		} else {
			// Execute single step
			step := item.AsStep()
			log.Printf("[Step %d/%d] Starting step %q on instance %q...", i+1, len(cfg.Workflow), step.Name, step.Instance)

			if callbacks != nil {
				callbacks.OnStepStart(i, 0, step.Name, "")
			}

			result, err := runStep(ctx, cfg, step)

			if callbacks != nil {
				callbacks.OnStepComplete(i, 0, step.Name, result, err)
			}

			if err != nil {
				return fmt.Errorf("step %q failed: %w", step.Name, err)
			}

			log.Printf("  -> Build finished with result: %s", result)
			if result != "SUCCESS" {
				return fmt.Errorf("step %q failed with result: %s", step.Name, result)
			}

			log.Printf("[Step %d/%d] Completed successfully.", i+1, len(cfg.Workflow))
		}
	}

	duration := time.Since(start)
	log.Printf("Workflow completed successfully in %s.", duration)
	return nil
}

// runStep executes a single step and returns the build result.
func runStep(ctx context.Context, cfg *config.Config, step config.Step) (string, error) {
	instanceCfg, ok := cfg.Instances[step.Instance]
	if !ok {
		return "", fmt.Errorf("unknown instance %q", step.Instance)
	}

	token, err := instanceCfg.GetToken()
	if err != nil {
		return "", fmt.Errorf("auth error: %w", err)
	}

	client := jenkins.NewClient(instanceCfg.URL, token)

	// 1. Trigger
	log.Printf("  -> [%s] Triggering job %s", step.Name, step.Job)
	queueItemURL, err := client.TriggerJob(ctx, step.Job, step.Params)
	if err != nil {
		return "", fmt.Errorf("failed to trigger: %w", err)
	}
	log.Printf("  -> [%s] Queued. Item: %s", step.Name, queueItemURL)

	// 2. Wait for Queue
	log.Printf("  -> [%s] Waiting for queue...", step.Name)
	buildURL, err := client.WaitForQueue(ctx, queueItemURL)
	if err != nil {
		return "", fmt.Errorf("failed waiting for queue: %w", err)
	}
	log.Printf("  -> [%s] Job started: %s", step.Name, buildURL)

	// 3. Wait for Build
	log.Printf("  -> [%s] Waiting for completion...", step.Name)
	result, err := client.WaitForBuild(ctx, buildURL)
	if err != nil {
		return "", fmt.Errorf("failed waiting for build: %w", err)
	}

	return result, nil
}

// runParallelGroup executes multiple steps in parallel.
// All steps must succeed for the group to succeed.
// If any step fails, remaining steps are cancelled (fail-fast).
func runParallelGroup(ctx context.Context, cfg *config.Config, steps []config.Step) ([]StepResult, error) {
	results := make([]StepResult, len(steps))
	var resultsMu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)

	for i, step := range steps {
		i, step := i, step // capture loop variables
		g.Go(func() error {
			result, err := runStep(gctx, cfg, step)

			resultsMu.Lock()
			results[i] = StepResult{
				StepName: step.Name,
				Result:   result,
				Error:    err,
			}
			resultsMu.Unlock()

			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}

			if result != "SUCCESS" {
				return fmt.Errorf("step %q failed with result: %s", step.Name, result)
			}

			return nil
		})
	}

	err := g.Wait()
	return results, err
}

// runParallelGroupWithCallbacks executes multiple steps in parallel with callback notifications.
func runParallelGroupWithCallbacks(ctx context.Context, cfg *config.Config, steps []config.Step, itemIndex int, callbacks WorkflowCallbacks) ([]StepResult, error) {
	results := make([]StepResult, len(steps))
	var resultsMu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)

	for i, step := range steps {
		i, step := i, step // capture loop variables
		g.Go(func() error {
			if callbacks != nil {
				callbacks.OnStepStart(itemIndex, i, step.Name, "")
			}

			result, err := runStep(gctx, cfg, step)

			resultsMu.Lock()
			results[i] = StepResult{
				StepName: step.Name,
				Result:   result,
				Error:    err,
			}
			resultsMu.Unlock()

			if callbacks != nil {
				callbacks.OnStepComplete(itemIndex, i, step.Name, result, err)
			}

			if err != nil {
				return fmt.Errorf("step %q: %w", step.Name, err)
			}

			if result != "SUCCESS" {
				return fmt.Errorf("step %q failed with result: %s", step.Name, result)
			}

			return nil
		})
	}

	err := g.Wait()
	return results, err
}
