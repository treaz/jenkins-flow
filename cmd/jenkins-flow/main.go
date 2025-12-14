package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/treaz/jenkins-flow/pkg/logger"
	"github.com/treaz/jenkins-flow/pkg/server"
)

func main() {
	// Define flags
	port := flag.Int("port", 32567, "Port to run the dashboard server on")
	instancesPath := flag.String("instances", "instances.yaml", "Path to instances configuration file")
	workflowsDir := flag.String("workflows-dir", "workflows,examples", "Directory containing workflow files")
	dbPath := flag.String("db-path", "", "Path to SQLite database file (default: ~/.config/jenkins-flow/jenkins-flow.db)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	trace := flag.Bool("trace", false, "Enable trace logging (includes HTTP dumps)")
	help := flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *help {
		printUsage()
		return
	}

	l := initLogger(*debug, *trace)
	startServer(*port, *instancesPath, *workflowsDir, *dbPath, l)
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
  jenkins-flow [options]

Options:
  -port int           Port to run the dashboard server on (default 32567)
  -instances string   Path to instances configuration file (default "instances.yaml")
  -workflows-dir string  Directory containing workflow files (default "workflows,examples")
  -db-path string     Path to SQLite database file (default "~/.config/jenkins-flow/jenkins-flow.db")
  -debug              Enable debug logging
  -trace              Enable trace logging (includes HTTP dumps)
  -help               Show this help message

Examples:
  jenkins-flow -port 3000
  jenkins-flow -instances my-instances.yaml
  jenkins-flow -db-path /custom/path/db.sqlite`)
}

func startServer(port int, instancesPath, workflowsDir, dbPath string, l *logger.Logger) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	workflowDirsList := strings.Split(workflowsDir, ",")
	srv := server.NewServer(port, instancesPath, workflowDirsList, dbPath, l)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
