package main

import (
	"context"
	"flag"
	"log"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/notifier"
	"github.com/treaz/jenkins-flow/pkg/workflow"
)

func main() {
	workflowPath := flag.String("workflow", "workflow.yaml", "Path to workflow configuration file")
	instancesPath := flag.String("instances", "instances.yaml", "Path to instances configuration file")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialize notifier (reads Slack config from env if available)
	n := notifier.NewFromEnv()

	// 1. Load Config
	cfg, err := config.Load(*instancesPath, *workflowPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 2. Run Workflow
	ctx := context.Background() // Can add signal handling here for graceful shutdown
	err = workflow.Run(ctx, cfg)

	// 3. Notify
	if err != nil {
		n.Notify(false, "Jenkins Flow", "Workflow FAILED: "+err.Error())
		log.Fatalf("Workflow failed: %v", err)
	} else {
		n.Notify(true, "Jenkins Flow", "Workflow Completed Successfully!")
		log.Println("Workflow finished successfully.")
	}
}
