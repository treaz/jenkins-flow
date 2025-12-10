package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/logger"
	"github.com/treaz/jenkins-flow/pkg/workflow"
)

// Server provides the HTTP server for the dashboard UI.
type Server struct {
	port          int
	instancesPath string
	workflowsDir  string
	state         *StateManager
	logger        *logger.Logger
	staticFS      fs.FS
	mu            sync.Mutex
	cancelFn      context.CancelFunc
}

// StaticFiles will be embedded at build time.
//
//go:embed static/*
var StaticFiles embed.FS

// NewServer creates a new dashboard server.
func NewServer(port int, instancesPath, workflowsDir string, l *logger.Logger) *Server {
	// Get the static subdirectory from embedded files
	staticFS, err := fs.Sub(StaticFiles, "static")
	if err != nil {
		log.Printf("Warning: Could not load embedded static files: %v", err)
	}

	return &Server{
		port:          port,
		instancesPath: instancesPath,
		workflowsDir:  workflowsDir,
		state:         NewStateManager(),
		logger:        l,
		staticFS:      staticFS,
	}
}

// WorkflowInfo describes an available workflow.
type WorkflowInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// StatusResponse is the response for the /api/status endpoint.
type StatusResponse struct {
	Running  bool           `json:"running"`
	Workflow *WorkflowState `json:"workflow,omitempty"`
}

// RunRequest is the request body for /api/run.
type RunRequest struct {
	Workflow string `json:"workflow"`
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/workflows", s.handleListWorkflows)
	mux.HandleFunc("/api/workflows/", s.handleWorkflowDefinition)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/run", s.handleRun)
	mux.HandleFunc("/api/stop", s.handleStop)
	mux.HandleFunc("/api/settings/log-level", s.handleLogLevel)

	// Static files (Vue app)
	if s.staticFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.staticFS)))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Jenkins Flow Dashboard</title></head>
<body>
<h1>Jenkins Flow Dashboard</h1>
<p>Static files not embedded. Run <code>npm run build</code> in the web directory.</p>
</body>
</html>`))
		})
	}

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting dashboard server on http://localhost%s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleListWorkflows returns available workflow files.
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflows := []WorkflowInfo{}

	// Look for workflow files in the workflows directory
	entries, err := os.ReadDir(s.workflowsDir)
	if err != nil {
		log.Printf("Error reading workflows directory: %v", err)
		http.Error(w, "Failed to read workflows directory", http.StatusInternalServerError)
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
			fullPath := filepath.Join(s.workflowsDir, name)

			// Parse the name from the file content
			workflowName, err := config.ParseWorkflowMeta(fullPath)
			if err != nil {
				log.Printf("Warning: Skipping invalid workflow file %q: %v", name, err)
				continue
			}

			workflows = append(workflows, WorkflowInfo{
				Name: workflowName,
				Path: fullPath,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

// handleWorkflowDefinition returns the static definition of a workflow for preview purposes.
func (s *Server) handleWorkflowDefinition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const prefix = "/api/workflows/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}

	requested := strings.TrimPrefix(r.URL.Path, prefix)
	if !strings.HasSuffix(requested, "/definition") {
		http.NotFound(w, r)
		return
	}

	requested = strings.TrimSuffix(requested, "/definition")
	requested = strings.TrimSuffix(requested, "/")

	if requested == "" {
		http.Error(w, "Workflow path is required", http.StatusBadRequest)
		return
	}

	workflowPath, err := url.PathUnescape(requested)
	if err != nil {
		http.Error(w, "Invalid workflow path", http.StatusBadRequest)
		return
	}

	workflowPath = filepath.Clean(workflowPath)
	workflowsRoot := filepath.Clean(s.workflowsDir)
	if !strings.HasPrefix(workflowPath, workflowsRoot+string(os.PathSeparator)) && workflowPath != workflowsRoot {
		http.Error(w, "Workflow path outside allowed directory", http.StatusForbidden)
		return
	}

	if stat, err := os.Stat(workflowPath); err != nil || stat.IsDir() {
		http.Error(w, "Workflow file not found", http.StatusNotFound)
		return
	}

	cfg, err := config.Load(s.instancesPath, workflowPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load workflow: %v", err), http.StatusBadRequest)
		return
	}

	definition := &WorkflowState{
		Name:   workflowPath,
		Status: StatusPending,
		Items:  s.configToStateItems(cfg),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(definition)
}

// handleStatus returns the current workflow execution status.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := StatusResponse{
		Running:  s.state.IsRunning(),
		Workflow: s.state.GetState(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRun starts a workflow execution.
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if already running
	if s.state.IsRunning() {
		http.Error(w, "A workflow is already running", http.StatusConflict)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	workflowPath := req.Workflow
	if workflowPath == "" {
		http.Error(w, "Workflow path is required", http.StatusBadRequest)
		return
	}

	// Load config
	cfg, err := config.Load(s.instancesPath, workflowPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusBadRequest)
		return
	}

	// Initialize state from config
	items := s.configToStateItems(cfg)
	s.state.StartWorkflow(workflowPath, items)

	// Run workflow in background
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelFn = cancel
	s.mu.Unlock()

	go s.runWorkflow(ctx, cfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handleStop stops a running workflow.
func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
		s.logger.Infof("Workflow stop requested by user")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
		return
	}

	http.Error(w, "No workflow running", http.StatusNotFound)
}

// LogLevelRequest is the request body for changing log level
type LogLevelRequest struct {
	Level string `json:"level"`
}

// handleLogLevel gets or sets the current log level
func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		level := s.logger.GetLevel().String()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"level": level})
		return
	}

	if r.Method == http.MethodPost {
		var req LogLevelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		lvl, err := logger.ParseLevel(req.Level)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid log level: %v", err), http.StatusBadRequest)
			return
		}

		s.logger.SetLevel(lvl)
		s.logger.Infof("Log level changed to %s", lvl.String())

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"level": lvl.String()})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// configToStateItems converts config workflow items to state items.
func (s *Server) configToStateItems(cfg *config.Config) []WorkflowItemState {
	items := make([]WorkflowItemState, len(cfg.Workflow))

	for i, item := range cfg.Workflow {
		if item.IsParallel() {
			steps := make([]StepState, len(item.Parallel.Steps))
			for j, step := range item.Parallel.Steps {
				steps[j] = StepState{
					Name:     step.Name,
					Instance: step.Instance,
					Job:      step.Job,
					Status:   StatusPending,
				}
			}
			items[i] = WorkflowItemState{
				IsParallel: true,
				Parallel: &ParallelGroupState{
					Name:   item.Parallel.Name,
					Steps:  steps,
					Status: StatusPending,
				},
			}
		} else {
			step := item.AsStep()
			items[i] = WorkflowItemState{
				IsParallel: false,
				Step: &StepState{
					Name:     step.Name,
					Instance: step.Instance,
					Job:      step.Job,
					Status:   StatusPending,
				},
			}
		}
	}

	return items
}

// runWorkflow executes the workflow and updates state.
func (s *Server) runWorkflow(ctx context.Context, cfg *config.Config) {
	defer func() {
		s.mu.Lock()
		s.cancelFn = nil
		s.mu.Unlock()
	}()

	// Create a state-aware runner
	err := workflow.RunWithCallbacks(ctx, cfg, s.logger, &workflowCallbacks{
		state: s.state,
	})

	if err != nil {
		s.state.CompleteWorkflow(false, err.Error())
	} else {
		s.state.CompleteWorkflow(true, "")
	}
}

// workflowCallbacks implements the callback interface for state updates.
type workflowCallbacks struct {
	state *StateManager
}

func (c *workflowCallbacks) OnStepStart(itemIndex, stepIndex int, name, buildURL string) {
	c.state.UpdateStepStatus(itemIndex, stepIndex, StatusRunning, "", "", buildURL)
}

func (c *workflowCallbacks) OnStepComplete(itemIndex, stepIndex int, name, result string, err error) {
	errMsg := ""
	status := StatusSuccess
	if err != nil {
		errMsg = err.Error()
		status = StatusFailed
	} else if result != "SUCCESS" {
		status = StatusFailed
	}
	c.state.UpdateStepStatus(itemIndex, stepIndex, status, result, errMsg, "")
}
