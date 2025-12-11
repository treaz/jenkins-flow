package workflow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/github"
	"github.com/treaz/jenkins-flow/pkg/jenkins"
	"github.com/treaz/jenkins-flow/pkg/logger"
	"golang.org/x/sync/errgroup"
)

// StepResult holds the result of a step execution.
type StepResult struct {
	StepName string
	Result   string
	Error    error
}

// Run executes the defined workflow, supporting both sequential and parallel steps.
func Run(ctx context.Context, cfg *config.Config, l *logger.Logger, skipPRCheck bool) error {
	l.Infof("Starting workflow execution...")
	start := time.Now()

	for i, item := range cfg.Workflow {
		if item.IsPRWait() {
			// Execute PR wait
			pr := item.WaitForPR
			target := describePRTarget(pr)
			l.Infof("[%d/%d] Waiting for %s (%s/%s) to be %s...",
				i+1, len(cfg.Workflow), target, pr.Owner, pr.Repo, pr.WaitFor)

			if skipPRCheck {
				l.Infof("Skipping PR check as requested by user.")
			} else {
				if err := runPRWait(ctx, cfg, pr, l, nil, i); err != nil {
					return fmt.Errorf("PR wait %q failed: %w", pr.Name, err)
				}
			}

			resolved := describeResolvedPR(pr)
			l.Infof("[%d/%d] %s is now %s. Continuing workflow...",
				i+1, len(cfg.Workflow), resolved, pr.WaitFor)
		} else if item.IsParallel() {
			// Execute parallel group
			groupName := item.Parallel.Name
			if groupName == "" {
				groupName = fmt.Sprintf("Parallel Group %d", i+1)
			}
			l.Infof("[%d/%d] Starting %s (%d steps)...", i+1, len(cfg.Workflow), groupName, len(item.Parallel.Steps))

			results, err := runParallelGroup(ctx, cfg, item.Parallel.Steps, l)
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
			l.Infof("[Step %d/%d] Starting step %q on instance %q...", i+1, len(cfg.Workflow), step.Name, step.Instance)

			result, err := runStep(ctx, cfg, step, l, nil, i, 0)
			if err != nil {
				return fmt.Errorf("step %q failed: %w", step.Name, err)
			}

			l.Infof("  -> Build finished with result: %s", result)
			if result != "SUCCESS" {
				return fmt.Errorf("step %q failed with result: %s", step.Name, result)
			}

			l.Infof("[Step %d/%d] Completed successfully.", i+1, len(cfg.Workflow))
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
	OnPRWaitStart(itemIndex int, pr *config.PRWait)
	OnPRWaitProgress(itemIndex int, pr *config.PRWait)
	OnPRWaitComplete(itemIndex int, pr *config.PRWait)
	OnPRWaitFailed(itemIndex int, pr *config.PRWait, err error)
}

// RunWithCallbacks executes the workflow with callback notifications.
func RunWithCallbacks(ctx context.Context, cfg *config.Config, l *logger.Logger, callbacks WorkflowCallbacks, skipPRCheck bool) error {
	l.Infof("Starting workflow execution...")
	start := time.Now()

	for i, item := range cfg.Workflow {
		if item.IsPRWait() {
			// Execute PR wait
			pr := item.WaitForPR
			target := describePRTarget(pr)
			l.Infof("[%d/%d] Waiting for %s (%s/%s) to be %s...",
				i+1, len(cfg.Workflow), target, pr.Owner, pr.Repo, pr.WaitFor)

			if skipPRCheck {
				l.Infof("Skipping PR check as requested by user.")
				if callbacks != nil {
					// We might want to indicate it was skipped or just mark complete?
					// Let's just mark complete immediately.
					callbacks.OnPRWaitStart(i, pr)
					callbacks.OnPRWaitComplete(i, pr)
				}
			} else {
				if err := runPRWait(ctx, cfg, pr, l, callbacks, i); err != nil {
					if callbacks != nil {
						callbacks.OnPRWaitFailed(i, pr, err)
					}
					return fmt.Errorf("PR wait %q failed: %w", pr.Name, err)
				}
				if callbacks != nil {
					callbacks.OnPRWaitComplete(i, pr)
				}
			}

			resolved := describeResolvedPR(pr)
			l.Infof("[%d/%d] %s is now %s. Continuing workflow...",
				i+1, len(cfg.Workflow), resolved, pr.WaitFor)
		} else if item.IsParallel() {
			// Execute parallel group
			groupName := item.Parallel.Name
			if groupName == "" {
				groupName = fmt.Sprintf("Parallel Group %d", i+1)
			}
			l.Infof("[%d/%d] Starting %s (%d steps)...", i+1, len(cfg.Workflow), groupName, len(item.Parallel.Steps))

			results, err := runParallelGroupWithCallbacks(ctx, cfg, item.Parallel.Steps, i, l, callbacks)
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
			l.Infof("[Step %d/%d] Starting step %q on instance %q...", i+1, len(cfg.Workflow), step.Name, step.Instance)

			if callbacks != nil {
				callbacks.OnStepStart(i, 0, step.Name, "")
			}

			result, err := runStep(ctx, cfg, step, l, callbacks, i, 0)

			if callbacks != nil {
				callbacks.OnStepComplete(i, 0, step.Name, result, err)
			}

			if err != nil {
				return fmt.Errorf("step %q failed: %w", step.Name, err)
			}

			l.Infof("  -> Build finished with result: %s", result)
			if result != "SUCCESS" {
				return fmt.Errorf("step %q failed with result: %s", step.Name, result)
			}

			l.Infof("[Step %d/%d] Completed successfully.", i+1, len(cfg.Workflow))
		}
	}

	duration := time.Since(start)
	l.Infof("Workflow completed successfully in %s.", duration)
	return nil
}

// runStep executes a single step and returns the build result.
func runStep(ctx context.Context, cfg *config.Config, step config.Step, l *logger.Logger, callbacks WorkflowCallbacks, itemIndex, stepIndex int) (string, error) {
	instanceCfg, ok := cfg.Instances[step.Instance]
	if !ok {
		return "", fmt.Errorf("unknown instance %q", step.Instance)
	}

	token, err := instanceCfg.GetToken()
	if err != nil {
		return "", fmt.Errorf("auth error: %w", err)
	}

	client := jenkins.NewClient(instanceCfg.URL, token, l)

	// 1. Trigger
	l.Infof("  -> [%s] Triggering job %s", step.Name, step.Job)
	queueItemURL, err := client.TriggerJob(ctx, step.Job, step.Params)
	if err != nil {
		return "", fmt.Errorf("failed to trigger: %w", err)
	}
	l.Infof("  -> [%s] Queued. Item: %s", step.Name, queueItemURL)

	// 2. Wait for Queue
	l.Infof("  -> [%s] Waiting for queue...", step.Name)
	buildURL, err := client.WaitForQueue(ctx, queueItemURL)
	if err != nil {
		return "", fmt.Errorf("failed waiting for queue: %w", err)
	}
	l.Infof("  -> [%s] Job started: %s", step.Name, buildURL)

	if callbacks != nil && buildURL != "" {
		callbacks.OnStepStart(itemIndex, stepIndex, step.Name, buildURL)
	}

	// 3. Wait for Build
	l.Infof("  -> [%s] Waiting for completion...", step.Name)
	result, err := client.WaitForBuild(ctx, buildURL)
	if err != nil {
		return "", fmt.Errorf("failed waiting for build: %w", err)
	}

	return result, nil
}

// runPRWait monitors a GitHub PR until it reaches the target state.
func runPRWait(ctx context.Context, cfg *config.Config, pr *config.PRWait, l *logger.Logger, callbacks WorkflowCallbacks, itemIndex int) error {
	if cfg.GitHub == nil {
		return fmt.Errorf("github configuration is required for wait_for_pr steps")
	}

	token, err := cfg.GitHub.GetToken()
	if err != nil {
		return fmt.Errorf("github auth error: %w", err)
	}

	client := github.NewClient(token, l)
	pollInterval := time.Duration(pr.PollSecs) * time.Second
	if pollInterval == 0 {
		pollInterval = 30 * time.Second
	}

	if callbacks != nil {
		callbacks.OnPRWaitStart(itemIndex, pr)
	}

	prNumber := pr.PRNumber
	if prNumber == 0 && pr.HeadBranch != "" {
		resolved, err := client.FindPRByBranch(ctx, pr.Owner, pr.Repo, pr.HeadBranch)
		if err != nil {
			return fmt.Errorf("failed to resolve branch %q: %w", pr.HeadBranch, err)
		}
		prNumber = resolved.Number
		pr.PRNumber = prNumber
		pr.ResolvedURL = resolved.HTMLURL
		pr.ResolvedTitle = resolved.Title
		l.Infof("  -> Resolved branch %q to PR #%d (%s)", pr.HeadBranch, prNumber, resolved.HTMLURL)
		if callbacks != nil {
			callbacks.OnPRWaitProgress(itemIndex, pr)
		}
	}

	if prNumber == 0 {
		return fmt.Errorf("no PR number resolved for wait step %q", pr.Name)
	}

	if pr.ResolvedURL == "" || pr.ResolvedTitle == "" {
		status, err := client.GetPRStatus(ctx, pr.Owner, pr.Repo, prNumber)
		if err != nil {
			return fmt.Errorf("failed to fetch PR #%d metadata: %w", prNumber, err)
		}
		pr.ResolvedURL = status.HTMLURL
		pr.ResolvedTitle = status.Title
		if callbacks != nil {
			callbacks.OnPRWaitProgress(itemIndex, pr)
		}
	}

	finalStatus, err := client.WaitForPRStatus(ctx, pr.Owner, pr.Repo, prNumber, pr.WaitFor, pollInterval)
	if err != nil {
		return err
	}
	if finalStatus != nil {
		pr.ResolvedURL = finalStatus.HTMLURL
		pr.ResolvedTitle = finalStatus.Title
		if callbacks != nil {
			callbacks.OnPRWaitProgress(itemIndex, pr)
		}
	}

	return nil
}

func describePRTarget(pr *config.PRWait) string {
	if pr == nil {
		return "PR"
	}
	if pr.PRNumber > 0 {
		return fmt.Sprintf("PR #%d", pr.PRNumber)
	}
	if pr.HeadBranch != "" {
		return fmt.Sprintf("PR on branch %q", pr.HeadBranch)
	}
	return "PR"
}

func describeResolvedPR(pr *config.PRWait) string {
	if pr == nil {
		return "PR"
	}
	if pr.PRNumber > 0 {
		return fmt.Sprintf("PR #%d", pr.PRNumber)
	}
	if pr.HeadBranch != "" {
		return fmt.Sprintf("PR on branch %q", pr.HeadBranch)
	}
	return "PR"
}

// runParallelGroup executes multiple steps in parallel.
func runParallelGroup(ctx context.Context, cfg *config.Config, steps []config.Step, l *logger.Logger) ([]StepResult, error) {
	results := make([]StepResult, len(steps))
	var resultsMu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)

	for i, step := range steps {
		i, step := i, step // capture loop variables
		g.Go(func() error {
			result, err := runStep(gctx, cfg, step, l, nil, 0, i)

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
func runParallelGroupWithCallbacks(ctx context.Context, cfg *config.Config, steps []config.Step, itemIndex int, l *logger.Logger, callbacks WorkflowCallbacks) ([]StepResult, error) {
	results := make([]StepResult, len(steps))
	var resultsMu sync.Mutex

	g, gctx := errgroup.WithContext(ctx)

	for i, step := range steps {
		i, step := i, step // capture loop variables
		g.Go(func() error {
			if callbacks != nil {
				callbacks.OnStepStart(itemIndex, i, step.Name, "")
			}

			result, err := runStep(gctx, cfg, step, l, callbacks, itemIndex, i)

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
