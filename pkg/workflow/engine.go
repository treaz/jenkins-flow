package workflow

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/jenkins"
)

// Run executes the defined workflow
func Run(ctx context.Context, cfg *config.Config) error {
	log.Println("Starting workflow execution...")
	start := time.Now()

	for i, step := range cfg.Workflow {
		log.Printf("[Step %d/%d] Starting step %q on instance %q...", i+1, len(cfg.Workflow), step.Name, step.Instance)

		instanceCfg, ok := cfg.Instances[step.Instance]
		if !ok {
			return fmt.Errorf("unknown instance %q in step %q", step.Instance, step.Name)
		}

		token, err := instanceCfg.GetToken()
		if err != nil {
			return fmt.Errorf("auth error for step %q: %w", step.Name, err)
		}

		client := jenkins.NewClient(instanceCfg.URL, token)

		// 1. Trigger
		log.Printf("  -> Triggering job %s", step.Job)
		queueItemURL, err := client.TriggerJob(ctx, step.Job, step.Params)
		if err != nil {
			return fmt.Errorf("failed to trigger step %q: %w", step.Name, err)
		}
		log.Printf("  -> Queued. Item: %s", queueItemURL)

		// 2. Wait for Queue
		log.Printf("  -> Waiting for queue...")
		buildURL, err := client.WaitForQueue(ctx, queueItemURL)
		if err != nil {
			return fmt.Errorf("failed waiting for queue in step %q: %w", step.Name, err)
		}
		log.Printf("  -> Job started: %s", buildURL)

		// 3. Wait for Build
		log.Printf("  -> Waiting for completion...")
		result, err := client.WaitForBuild(ctx, buildURL)
		if err != nil {
			return fmt.Errorf("failed waiting for build in step %q: %w", step.Name, err)
		}
		log.Printf("  -> Build finished with result: %s", result)

		if result != "SUCCESS" {
			return fmt.Errorf("step %q failed with result: %s", step.Name, result)
		}

		log.Printf("[Step %d/%d] Completed successfully.", i+1, len(cfg.Workflow))
	}

	duration := time.Since(start)
	log.Printf("Workflow completed successfully in %s.", duration)
	return nil
}
