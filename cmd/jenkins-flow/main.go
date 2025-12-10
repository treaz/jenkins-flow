package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/logger"
	"github.com/treaz/jenkins-flow/pkg/notifier"
	"github.com/treaz/jenkins-flow/pkg/server"
	"github.com/treaz/jenkins-flow/pkg/workflow"
)

func main() {
	// Define subcommands
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	runWorkflowPath := runCmd.String("workflow", "workflow.yaml", "Path to workflow configuration file")
	runInstancesPath := runCmd.String("instances", "instances.yaml", "Path to instances configuration file")
	runDebug := runCmd.Bool("debug", false, "Enable debug logging")
	runTrace := runCmd.Bool("trace", false, "Enable trace logging (includes HTTP dumps)")

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	servePort := serveCmd.Int("port", 32567, "Port to run the dashboard server on")
	serveInstancesPath := serveCmd.String("instances", "instances.yaml", "Path to instances configuration file")
	serveWorkflowsDir := serveCmd.String("workflows-dir", "workflows", "Directory containing workflow files")
	serveDebug := serveCmd.Bool("debug", false, "Enable debug logging")
	serveTrace := serveCmd.Bool("trace", false, "Enable trace logging (includes HTTP dumps)")

	// Check for subcommand
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		runCmd.Parse(os.Args[2:])
		l := initLogger(*runDebug, *runTrace)
		runWorkflow(*runInstancesPath, *runWorkflowPath, l)

	case "serve":
		serveCmd.Parse(os.Args[2:])
		l := initLogger(*serveDebug, *serveTrace)
		startServer(*servePort, *serveInstancesPath, *serveWorkflowsDir, l)

	case "help", "-h", "--help":
		printUsage()

	default:
		// For backward compatibility, treat no subcommand as "run"
		// Re-parse with default flags
		legacyFlags := flag.NewFlagSet("jenkins-flow", flag.ExitOnError)
		workflowPath := legacyFlags.String("workflow", "workflow.yaml", "Path to workflow configuration file")
		instancesPath := legacyFlags.String("instances", "instances.yaml", "Path to instances configuration file")
		debug := legacyFlags.Bool("debug", false, "Enable debug logging")
		trace := legacyFlags.Bool("trace", false, "Enable trace logging")
		legacyFlags.Parse(os.Args[1:])
		l := initLogger(*debug, *trace)
		runWorkflow(*instancesPath, *workflowPath, l)
	}
}

func initLogger(debug, trace bool) *logger.Logger {
	level := logger.Info
	if trace {
		level = logger.Trace
	} else if debug {
		level = logger.Debug
	}
	return logger.New(level)
}

func printUsage() {
	fmt.Println(`Jenkins Flow - Workflow Orchestration Tool

Usage:
  jenkins-flow <command> [options]

Commands:
  run      Run a workflow (default if no command specified)
  serve    Start the web dashboard server
  help     Show this help message

Run Options:
  -workflow string    Path to workflow configuration file (default "workflow.yaml")
  -instances string   Path to instances configuration file (default "instances.yaml")

Serve Options:
  -port int           Port to run the dashboard server on (default 32567)
  -instances string   Path to instances configuration file (default "instances.yaml")
  -workflows-dir string  Directory containing workflow files (default "workflows")

Examples:
  jenkins-flow run -workflow deploy.yaml
  jenkins-flow serve -port 3000
  jenkins-flow -workflow workflow.yaml  (legacy syntax, runs workflow)`)
}

func runWorkflow(instancesPath, workflowPath string, l *logger.Logger) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialize notifier (reads Slack config from env if available)
	n := notifier.NewFromEnv()

	// 1. Load Config
	cfg, err := config.Load(instancesPath, workflowPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// 2. Run Workflow
	ctx := context.Background() // Can add signal handling here for graceful shutdown
	err = workflow.Run(ctx, cfg, l)

	// 3. Notify
	if err != nil {
		n.Notify(false, "Jenkins Flow", "Workflow FAILED: "+err.Error())
		log.Fatalf("Workflow failed: %v", err)
	} else {
		n.Notify(true, "Jenkins Flow", "Workflow Completed Successfully!")
		log.Println("Workflow finished successfully.")
	}
}

func startServer(port int, instancesPath, workflowsDir string, l *logger.Logger) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	srv := server.NewServer(port, instancesPath, workflowsDir, l)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
