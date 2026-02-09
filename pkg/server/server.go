package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/treaz/jenkins-flow/pkg/api"
	"github.com/treaz/jenkins-flow/pkg/config"
	"github.com/treaz/jenkins-flow/pkg/database"
	"github.com/treaz/jenkins-flow/pkg/logger"
	"github.com/treaz/jenkins-flow/pkg/notifier"
	"github.com/treaz/jenkins-flow/pkg/settings"
	"github.com/treaz/jenkins-flow/pkg/workflow"
)

// Server provides the HTTP server for the dashboard UI.
type Server struct {
	port          int
	instancesPath string
	workflowDirs  []string
	state         *StateManager
	logger        *logger.Logger
	staticFS      fs.FS
	mu            sync.Mutex
	cancelFn      context.CancelFunc
	db            *database.DB
	dbPath        string
	currentRunID  int64
}

// StaticFiles will be embedded at build time.
//
//go:embed static/*
var StaticFiles embed.FS

// NewServer creates a new dashboard server.
func NewServer(port int, instancesPath string, workflowDirs []string, dbPath string, l *logger.Logger) *Server {
	// Get the static subdirectory from embedded files
	staticFS, err := fs.Sub(StaticFiles, "static")
	if err != nil {
		log.Printf("Warning: Could not load embedded static files: %v", err)
	}

	// Determine database path
	if dbPath == "" {
		dbPath, err = settings.GetDefaultDBPath()
		if err != nil {
			l.Errorf("Failed to get default database path: %v", err)
			dbPath = "jenkins-flow.db" // Fallback
		}
	}

	// Initialize database
	db, err := database.NewDB(dbPath)
	if err != nil {
		l.Errorf("Failed to initialize database: %v", err)
		// Don't fail server startup, just log the error
	}

	return &Server{
		port:          port,
		instancesPath: instancesPath,
		workflowDirs:  workflowDirs,
		state:         NewStateManager(),
		logger:        l,
		staticFS:      staticFS,
		db:            db,
		dbPath:        dbPath,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// API routes
	api.HandlerFromMux(s, r)

	// Swagger UI
	r.Get("/api/openapi.json", s.handleOpenAPISpec)
	r.Get("/swagger", s.handleSwaggerUI)

	// Static files (Vue app)
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the file exists in static FS, otherwise serve index.html (SPA)
			path := r.URL.Path
			if path == "/" {
				fileServer.ServeHTTP(w, r)
				return
			}

			// Try to open the file to see if it exists
			f, err := s.staticFS.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}

			// Not found, serve index.html for SPA routing
			// Re-open index.html
			index, err := s.staticFS.Open("index.html")
			if err != nil {
				http.Error(w, "Index not found", http.StatusInternalServerError)
				return
			}
			defer index.Close()
			stat, _ := index.Stat()
			if seeker, ok := index.(io.ReadSeeker); ok {
				http.ServeContent(w, r, "index.html", stat.ModTime(), seeker)
			} else {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}))
	} else {
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
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
	return http.ListenAndServe(addr, r)
}

// ListWorkflows returns available workflow files.
func (s *Server) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := []api.WorkflowInfo{}

	for _, dir := range s.workflowDirs {
		// Look for workflow files in the directory
		entries, err := os.ReadDir(dir)
		if err != nil {
			// Just log warning if one dir fails, don't fail entire request
			log.Printf("Warning: Error reading workflows directory %q: %v", dir, err)
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() && (strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")) {
				fullPath := filepath.Join(dir, name)

				// Parse the name from the file content
				workflowName, err := config.ParseWorkflowMeta(fullPath)
				if err != nil {
					// Include invalid workflows in list with error
					workflows = append(workflows, api.WorkflowInfo{
						Name:  strPtr(name),
						Path:  strPtr(fullPath),
						Valid: boolPtr(false),
						Error: strPtr(err.Error()),
					})
					continue
				}

				// Validate the complete workflow
				_, validationErr := config.Load(s.instancesPath, fullPath)
				if validationErr != nil {
					workflows = append(workflows, api.WorkflowInfo{
						Name:  strPtr(workflowName),
						Path:  strPtr(fullPath),
						Valid: boolPtr(false),
						Error: strPtr(validationErr.Error()),
					})
				} else {
					workflows = append(workflows, api.WorkflowInfo{
						Name:  strPtr(workflowName),
						Path:  strPtr(fullPath),
						Valid: boolPtr(true),
						Error: nil,
					})
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

// GetWorkflowDefinition returns the static definition of a workflow for preview purposes.
func (s *Server) GetWorkflowDefinition(w http.ResponseWriter, r *http.Request, name string) {
	workflowPath, err := url.PathUnescape(name)
	if err != nil {
		http.Error(w, "Invalid workflow path", http.StatusBadRequest)
		return
	}

	workflowPath = filepath.Clean(workflowPath)

	allowed := false
	for _, dir := range s.workflowDirs {
		workflowsRoot := filepath.Clean(dir)
		if strings.HasPrefix(workflowPath, workflowsRoot+string(os.PathSeparator)) || workflowPath == workflowsRoot {
			allowed = true
			break
		}
	}

	if !allowed {
		http.Error(w, "Workflow path outside allowed directories", http.StatusForbidden)
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

	s.applyInputSubstitutions(cfg)

	// Helper to convert config items to initial internal state, then to API state
	internalItems := s.configToStateItems(cfg)
	// We need to construct a "dummy" pending state to convert to API response
	dummyState := &WorkflowState{
		Name:      workflowPath,
		Status:    StatusPending,
		Inputs:    cfg.Inputs,
		Items:     internalItems,
		StartedAt: nil,
	}

	response := s.internalToAPI(dummyState)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetStatus returns the current workflow execution status.
func (s *Server) GetStatus(w http.ResponseWriter, r *http.Request) {
	internalState := s.state.GetState()
	var apiWorkflow *api.WorkflowState
	if internalState != nil {
		apiWorkflow = s.internalToAPI(internalState)
	}

	running := s.state.IsRunning()
	resp := api.StatusResponse{
		Running:  &running,
		Workflow: apiWorkflow,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// RunWorkflow starts a workflow execution.
func (s *Server) RunWorkflow(w http.ResponseWriter, r *http.Request) {
	// Check if already running
	if s.state.IsRunning() {
		http.Error(w, "A workflow is already running", http.StatusConflict)
		return
	}

	var req api.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Workflow == nil || *req.Workflow == "" {
		http.Error(w, "Workflow path is required", http.StatusBadRequest)
		return
	}
	workflowPath := *req.Workflow

	// Load config
	cfg, err := config.Load(s.instancesPath, workflowPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load config: %v", err), http.StatusBadRequest)
		return
	}

	// Update inputs if provided
	if req.Inputs != nil && len(*req.Inputs) > 0 {
		newInputs := *req.Inputs
		if cfg.Inputs == nil {
			cfg.Inputs = make(map[string]string)
		}

		// Update persistent file if values changed
		changed := false
		for k, v := range newInputs {
			if cfg.Inputs[k] != v {
				cfg.Inputs[k] = v
				changed = true
			}
		}

		if changed {
			if err := s.updateWorkflowFile(workflowPath, cfg.Inputs); err != nil {
				s.logger.Errorf("Failed to update workflow file: %v", err)
				// Continue running even if persistence fails?
				// The user specifically asked for persistence. Let's error or warn.
				// For now warn but continue with in-memory value.
			}
		}
	}

	s.applyInputSubstitutions(cfg)

	// Initialize state from config
	items := s.configToStateItems(cfg)
	s.state.StartWorkflow(workflowPath, cfg.Inputs, items)

	// Run workflow in background
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancelFn = cancel
	s.mu.Unlock()

	// Parse optional skipPRCheck
	skipPRCheck := false
	if req.SkipPRCheck != nil && *req.SkipPRCheck {
		skipPRCheck = true
	}

	go s.runWorkflow(ctx, cfg, workflowPath, skipPRCheck)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// updateWorkflowFile updates the workflow YAML file with new inputs without destroying comments.
func (s *Server) updateWorkflowFile(path string, inputs map[string]string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	text := string(content)

	// Helper to simple replace value for a key
	// Looks for "  key: old_value" or "key: old_value"
	// We want to be careful not to match partial keys or keys in specific structures if possible.
	// But assuming inputs are likely unique or we rely on them being in "inputs:" block is hard with regex alone efficiently without parsing.
	// Allow simple replacement for now as requested "very specific replace on the line".
	// Match: (whitespace)(key)(: )(value)(possible comment)(newline)
	// We only have the NEW value. We don't know the OLD value easily unless we look at loaded cfg (which we have).

	for key, newVal := range inputs {
		// Regex to find the key and replace its value.
		// Supports: "  key: value"
		// Capture group 1: leading whitespace
		// Capture group 2: key + colon + space
		// Capture group 3: old value (non-greedy)
		// Capture group 4: comment (optional)
		// Note: We assume strictly simple strings for input values (which they are)
		pattern := fmt.Sprintf(`(?m)^(\s*%s:\s*)(.+?)(\s*#.*)?$`, regexp.QuoteMeta(key))
		re := regexp.MustCompile(pattern)

		text = re.ReplaceAllStringFunc(text, func(match string) string {
			// Check if this looks like it's inside inputs block?
			// Hard to know context with regex.
			// But usually unique enough keys reduce risk.
			// If key is "name", it might match workflow name.
			// So we should verify if the key exists in Inputs map.

			// To allow comments preservation, we reconstruct the line.
			parts := re.FindStringSubmatch(match)
			if len(parts) < 3 {
				return match
			}
			prefix := parts[1]
			// value := parts[2] -- old value
			comment := ""
			if len(parts) > 3 {
				comment = parts[3]
			}

			return fmt.Sprintf("%s%s%s", prefix, newVal, comment)
		})
	}

	return os.WriteFile(path, []byte(text), 0644)
}

// StopWorkflow stops a running workflow.
func (s *Server) StopWorkflow(w http.ResponseWriter, r *http.Request) {
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

// GetLogLevel gets the current log level
func (s *Server) GetLogLevel(w http.ResponseWriter, r *http.Request) {
	level := s.logger.GetLevel().String()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.LogLevelRequest{Level: &level}) // Reusing Request struct for response as it matches shape
}

// SetLogLevel sets the current log level
func (s *Server) SetLogLevel(w http.ResponseWriter, r *http.Request) {
	var req api.LogLevelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Level == nil {
		http.Error(w, "Level is required", http.StatusBadRequest)
		return
	}

	lvl, err := logger.ParseLevel(*req.Level)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid log level: %v", err), http.StatusBadRequest)
		return
	}

	s.logger.SetLevel(lvl)
	s.logger.Infof("Log level changed to %s", lvl.String())

	levelStr := lvl.String()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(api.LogLevelRequest{Level: &levelStr})
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
				IsPRWait:   false,
				Parallel: &ParallelGroupState{
					Name:   item.Parallel.Name,
					Steps:  steps,
					Status: StatusPending,
				},
			}
		} else if item.IsPRWait() {
			pr := item.WaitForPR
			htmlURL := ""
			if pr.PRNumber > 0 {
				htmlURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d", pr.Owner, pr.Repo, pr.PRNumber)
			}
			items[i] = WorkflowItemState{
				IsParallel: false,
				IsPRWait:   true,
				PRWait: &PRWaitState{
					Name:       pr.Name,
					Owner:      pr.Owner,
					Repo:       pr.Repo,
					HeadBranch: pr.HeadBranch,
					PRNumber:   pr.PRNumber,
					WaitFor:    pr.WaitFor,
					Status:     StatusPending,
					HTMLURL:    htmlURL,
					Title:      pr.ResolvedTitle,
				},
			}
		} else {
			step := item.AsStep()
			items[i] = WorkflowItemState{
				IsParallel: false,
				IsPRWait:   false,
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

func (s *Server) applyInputSubstitutions(cfg *config.Config) {
	if cfg == nil || len(cfg.Inputs) == 0 {
		return
	}

	for i := range cfg.Workflow {
		item := &cfg.Workflow[i]
		if !item.IsPRWait() || item.WaitForPR == nil {
			continue
		}

		pr := item.WaitForPR
		pr.Name = substituteIfTemplate(pr.Name, cfg.Inputs)
		pr.Owner = substituteIfTemplate(pr.Owner, cfg.Inputs)
		pr.Repo = substituteIfTemplate(pr.Repo, cfg.Inputs)
		pr.HeadBranch = substituteIfTemplate(pr.HeadBranch, cfg.Inputs)
		pr.WaitFor = substituteIfTemplate(pr.WaitFor, cfg.Inputs)
	}
}

func substituteIfTemplate(value string, inputs map[string]string) string {
	if value == "" || len(inputs) == 0 || !strings.Contains(value, "${") {
		return value
	}

	return config.Substitute(value, inputs)
}

// runWorkflow executes the workflow and updates state.
func (s *Server) runWorkflow(ctx context.Context, cfg *config.Config, workflowPath string, skipPRCheck bool) {
	defer func() {
		s.mu.Lock()
		s.cancelFn = nil
		s.mu.Unlock()
	}()

	start := time.Now()
	notify := notifier.NewFromWebhook(cfg.SlackWebhook)

	if !notify.HasSlack() {
		s.logger.Infof("WARN: Slack notifications disabled for workflow %q (define slack_webhook)", workflowPath)
	}

	displayName := cfg.Name
	if displayName == "" {
		displayName = filepath.Base(workflowPath)
	}
	if displayName == "" {
		displayName = "Workflow"
	}

	// Read workflow YAML content for snapshot
	configSnapshot := ""
	if content, err := os.ReadFile(workflowPath); err == nil {
		configSnapshot = string(content)
	} else {
		s.logger.Infof("WARNING: Failed to read workflow file for snapshot: %v", err)
	}

	// Create database record if database is available
	var runID int64
	if s.db != nil {
		var err error
		runID, err = s.db.CreateRun(cfg.Name, workflowPath, configSnapshot, cfg.Inputs, skipPRCheck)
		if err != nil {
			s.logger.Errorf("Failed to create workflow run record: %v", err)
			// Continue execution even if database write fails
		} else {
			s.mu.Lock()
			s.currentRunID = runID
			s.mu.Unlock()
			s.logger.Infof("Created workflow run record with ID: %d", runID)
		}
	}

	// Create a state-aware runner
	err := workflow.RunWithCallbacks(ctx, cfg, s.logger, &workflowCallbacks{
		state: s.state,
	}, skipPRCheck)

	duration := time.Since(start)

	// Determine final status
	finalStatus := "success"
	if err != nil {
		if ctx.Err() == context.Canceled {
			finalStatus = "stopped"
		} else {
			finalStatus = "failed"
		}
	}

	// Update database record if available
	if s.db != nil && runID > 0 {
		if dbErr := s.db.UpdateRunComplete(runID, finalStatus, time.Now()); dbErr != nil {
			s.logger.Errorf("Failed to update workflow run record: %v", dbErr)
		}
	}

	if err != nil {
		s.state.CompleteWorkflow(false, err.Error())
		notify.Notify(false, displayName, fmt.Sprintf("Failed after %s: %v", duration.Round(time.Second), err))
	} else {
		s.state.CompleteWorkflow(true, "")
		notify.Notify(true, displayName, fmt.Sprintf("Completed successfully in %s", duration.Round(time.Second)))
	}
}

// Helper functions for API conversion

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func (s *Server) internalToAPI(state *WorkflowState) *api.WorkflowState {
	items := make([]api.WorkflowItemState, len(state.Items))
	for i, item := range state.Items {
		items[i] = s.internalItemToAPI(item)
	}

	st := string(state.Status)
	return &api.WorkflowState{
		Name:   strPtr(state.Name),
		Status: strPtr(st),
		Inputs: &state.Inputs,
		Items:  &items,
	}
}

func (s *Server) internalItemToAPI(item WorkflowItemState) api.WorkflowItemState {
	res := api.WorkflowItemState{
		IsParallel: boolPtr(item.IsParallel),
		IsPRWait:   boolPtr(item.IsPRWait),
	}

	if item.Step != nil {
		res.Step = s.internalStepToAPI(item.Step)
	}

	if item.Parallel != nil {
		res.Parallel = s.internalParallelToAPI(item.Parallel)
	}

	if item.PRWait != nil {
		res.PrWait = s.internalPRWaitToAPI(item.PRWait)
	}

	return res
}

func (s *Server) internalStepToAPI(step *StepState) *api.StepState {
	st := string(step.Status)
	return &api.StepState{
		Name:     strPtr(step.Name),
		Instance: strPtr(step.Instance),
		Job:      strPtr(step.Job),
		Status:   strPtr(st),
		Result:   strPtr(step.Result),
		Error:    strPtr(step.Error),
		BuildUrl: strPtr(step.BuildURL),
	}
}

func (s *Server) internalParallelToAPI(p *ParallelGroupState) *api.ParallelGroupState {
	steps := make([]api.StepState, len(p.Steps))
	for i, step := range p.Steps {
		steps[i] = *s.internalStepToAPI(&step)
	}

	st := string(p.Status)
	return &api.ParallelGroupState{
		Name:   strPtr(p.Name),
		Status: strPtr(st),
		Steps:  &steps,
	}
}

func (s *Server) internalPRWaitToAPI(pr *PRWaitState) *api.PRWaitState {
	st := string(pr.Status)
	return &api.PRWaitState{
		Name:       strPtr(pr.Name),
		Owner:      strPtr(pr.Owner),
		Repo:       strPtr(pr.Repo),
		HeadBranch: strPtr(pr.HeadBranch),
		PrNumber:   intPtr(pr.PRNumber),
		WaitFor:    strPtr(pr.WaitFor),
		Status:     strPtr(st),
		HtmlUrl:    strPtr(pr.HTMLURL),
		Title:      strPtr(pr.Title),
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

func (c *workflowCallbacks) OnPRWaitStart(itemIndex int, pr *config.PRWait) {
	if pr == nil {
		return
	}
	c.state.StartPRWait(itemIndex, pr.Name, pr.Owner, pr.Repo, pr.HeadBranch, pr.WaitFor, pr.PRNumber, pr.ResolvedURL, pr.ResolvedTitle)
}

func (c *workflowCallbacks) OnPRWaitProgress(itemIndex int, pr *config.PRWait) {
	if pr == nil {
		return
	}
	c.state.UpdatePRWaitMetadata(itemIndex, pr.PRNumber, pr.ResolvedURL, pr.ResolvedTitle)
}

func (c *workflowCallbacks) OnPRWaitComplete(itemIndex int, pr *config.PRWait) {
	if pr != nil {
		c.state.UpdatePRWaitMetadata(itemIndex, pr.PRNumber, pr.ResolvedURL, pr.ResolvedTitle)
	}
	c.state.CompletePRWait(itemIndex)
}

func (c *workflowCallbacks) OnPRWaitFailed(itemIndex int, pr *config.PRWait, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	if pr != nil {
		c.state.UpdatePRWaitMetadata(itemIndex, pr.PRNumber, pr.ResolvedURL, pr.ResolvedTitle)
	}
	c.state.FailPRWait(itemIndex, errMsg)
}

// handleOpenAPISpec serves the OpenAPI specification as JSON
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	spec, err := api.GetSwagger()
	if err != nil {
		http.Error(w, "Error loading spec", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spec)
}

// handleSwaggerUI serves the Swagger UI HTML page
func (s *Server) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js" crossorigin></script>
<script>
  window.onload = () => {
    window.ui = SwaggerUIBundle({
      url: '/api/openapi.json',
      dom_id: '#swagger-ui',
    });
  };
</script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// GetHistory lists workflow run history with optional filters.
func (s *Server) GetHistory(w http.ResponseWriter, r *http.Request, params api.GetHistoryParams) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	// Set defaults
	limit := 50
	offset := 0
	workflowPath := ""
	status := ""

	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}
	if params.WorkflowPath != nil {
		workflowPath = *params.WorkflowPath
	}
	if params.Status != nil {
		status = *params.Status
	}

	runs, err := s.db.GetRuns(limit, offset, workflowPath, status)
	if err != nil {
		s.logger.Errorf("Failed to get workflow runs: %v", err)
		http.Error(w, "Failed to retrieve workflow runs", http.StatusInternalServerError)
		return
	}

	// Convert to API format
	apiRuns := make([]api.WorkflowRun, len(runs))
	for i, run := range runs {
		apiRuns[i] = api.WorkflowRun{
			Id:             &run.ID,
			WorkflowName:   &run.WorkflowName,
			WorkflowPath:   &run.WorkflowPath,
			StartTime:      &run.StartTime,
			EndTime:        run.EndTime,
			Status:         &run.Status,
			Inputs:         &run.Inputs,
			ConfigSnapshot: &run.ConfigSnapshot,
			SkipPrCheck:    &run.SkipPRCheck,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiRuns)
}

// GetHistoryRun retrieves a specific workflow run by ID.
func (s *Server) GetHistoryRun(w http.ResponseWriter, r *http.Request, id int) {
	if s.db == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	run, err := s.db.GetRun(int64(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Workflow run not found", http.StatusNotFound)
		} else {
			s.logger.Errorf("Failed to get workflow run: %v", err)
			http.Error(w, "Failed to retrieve workflow run", http.StatusInternalServerError)
		}
		return
	}

	// Convert to API format
	apiRun := api.WorkflowRun{
		Id:             &run.ID,
		WorkflowName:   &run.WorkflowName,
		WorkflowPath:   &run.WorkflowPath,
		StartTime:      &run.StartTime,
		EndTime:        run.EndTime,
		Status:         &run.Status,
		Inputs:         &run.Inputs,
		ConfigSnapshot: &run.ConfigSnapshot,
		SkipPrCheck:    &run.SkipPRCheck,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiRun)
}

// GetDBPath returns the current database path.
func (s *Server) GetDBPath(w http.ResponseWriter, r *http.Request) {
	path := s.dbPath
	resp := api.DBPathResponse{
		Path: &path,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// SetDBPath updates the database path in settings.
func (s *Server) SetDBPath(w http.ResponseWriter, r *http.Request) {
	var req api.DBPathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == nil || *req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	newPath := *req.Path

	// Save to settings
	settings, err := settings.Load()
	if err != nil {
		s.logger.Errorf("Failed to load settings: %v", err)
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	settings.DBPath = newPath
	if err := settings.Save(); err != nil {
		s.logger.Errorf("Failed to save settings: %v", err)
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	s.logger.Infof("Database path updated to: %s (will take effect on restart)", newPath)

	resp := api.DBPathResponse{
		Path: &newPath,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
