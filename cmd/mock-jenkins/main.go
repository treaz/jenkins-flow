// mock-jenkins is a lightweight HTTP server that simulates the Jenkins REST API
// endpoints used by jenkins-flow, enabling local smoke testing without a real
// Jenkins instance.
//
// Simulated endpoints:
//
//	POST /job/.../build[WithParameters]  → queues a fake job, returns Location header
//	GET  /queue/item/{id}/api/json       → returns build URL once queue delay passes
//	GET  /job/.../{n}/api/json          → returns build status / result
//
// Usage:
//
//	go run ./cmd/mock-jenkins [flags]
//
// Flags:
//
//	-port int              Port to listen on (default 9090)
//	-queue-delay duration  How long a job stays in the queue before starting (default 2s)
//	-build-duration duration  How long the build "runs" before completing (default 5s)
//	-result string         Build result to return: SUCCESS, FAILURE, UNSTABLE (default SUCCESS)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// queueItem represents a job waiting in the queue.
type queueItem struct {
	id          int64
	buildID     int64
	triggeredAt time.Time
}

// build represents a running or completed build.
type build struct {
	id        int64
	jobPath   string // e.g. /job/utils/echo
	startedAt time.Time
}

var (
	mu           sync.Mutex
	queueItems   = map[int64]*queueItem{}
	builds       = map[int64]*build{}
	queueCounter atomic.Int64
	buildCounter atomic.Int64

	// CLI-configurable behaviour
	listenPort    int
	queueDelay    time.Duration
	buildDuration time.Duration
	buildResult   string
)

func main() {
	flag.IntVar(&listenPort, "port", 9090, "Port to listen on")
	flag.DurationVar(&queueDelay, "queue-delay", 2*time.Second, "How long jobs wait in queue before starting")
	flag.DurationVar(&buildDuration, "build-duration", 5*time.Second, "How long each build takes to complete")
	flag.StringVar(&buildResult, "result", "SUCCESS", "Build result returned on completion (SUCCESS, FAILURE, UNSTABLE)")
	flag.Parse()

	log.Printf("Mock Jenkins server")
	log.Printf("  Listening on    : http://localhost:%d", listenPort)
	log.Printf("  Queue delay     : %s", queueDelay)
	log.Printf("  Build duration  : %s", buildDuration)
	log.Printf("  Build result    : %s", buildResult)
	log.Printf("")
	log.Printf("Configure instances.yaml:")
	log.Printf("  instances:")
	log.Printf("    mock:")
	log.Printf("      url: http://localhost:%d", listenPort)
	log.Printf("      token: ignored:token")

	http.HandleFunc("/", route)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", listenPort), nil))
}

func route(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Printf("%-6s %s", r.Method, path)

	switch {
	// Trigger: POST /job/.../build or /buildWithParameters
	case r.Method == http.MethodPost &&
		(strings.HasSuffix(path, "/build") || strings.HasSuffix(path, "/buildWithParameters")):
		handleTrigger(w, r)

	// Queue poll: GET /queue/item/{id}/api/json
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/queue/item/") && strings.HasSuffix(path, "/api/json"):
		handleQueuePoll(w, r)

	// Build poll: GET /job/.../{n}/api/json
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/api/json"):
		handleBuildPoll(w, r)

	default:
		http.NotFound(w, r)
	}
}

// handleTrigger responds to a job trigger request.
// It creates a queue item and returns its URL in the Location header.
func handleTrigger(w http.ResponseWriter, r *http.Request) {
	// Strip /build or /buildWithParameters suffix to get the job path
	jobPath := r.URL.Path
	if idx := strings.LastIndex(jobPath, "/build"); idx >= 0 {
		jobPath = jobPath[:idx]
	}

	qID := queueCounter.Add(1)
	bID := buildCounter.Add(1)

	mu.Lock()
	queueItems[qID] = &queueItem{
		id:          qID,
		buildID:     bID,
		triggeredAt: time.Now(),
	}
	builds[bID] = &build{
		id:        bID,
		jobPath:   jobPath,
		startedAt: time.Now().Add(queueDelay),
	}
	mu.Unlock()

	// Log any parameters that were passed
	if err := r.ParseForm(); err == nil && len(r.Form) > 0 {
		log.Printf("  params: %v", r.Form)
	}

	location := fmt.Sprintf("http://localhost:%d/queue/item/%d/", listenPort, qID)
	log.Printf("  queued → item %d, build %d", qID, bID)
	w.Header().Set("Location", location)
	w.WriteHeader(http.StatusCreated)
}

// handleQueuePoll returns the build URL once the queue delay has elapsed.
func handleQueuePoll(w http.ResponseWriter, r *http.Request) {
	// Path: /queue/item/{id}/api/json
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// parts = ["queue", "item", "{id}", "api", "json"]
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	qID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.Error(w, "invalid queue id", http.StatusBadRequest)
		return
	}

	mu.Lock()
	item, ok := queueItems[qID]
	mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if time.Since(item.triggeredAt) < queueDelay {
		// Still queued — no executable yet
		log.Printf("  queue item %d: waiting...", qID)
		json.NewEncoder(w).Encode(map[string]any{
			"id":         qID,
			"cancelled":  false,
			"executable": nil,
		})
		return
	}

	// Queue delay passed — expose the build URL
	mu.Lock()
	b, hasBuild := builds[item.buildID]
	mu.Unlock()

	if !hasBuild {
		http.NotFound(w, r)
		return
	}

	buildURL := fmt.Sprintf("http://localhost:%d%s/%d/", listenPort, b.jobPath, b.id)
	log.Printf("  queue item %d: started → %s", qID, buildURL)
	json.NewEncoder(w).Encode(map[string]any{
		"id":        qID,
		"cancelled": false,
		"executable": map[string]any{
			"url":    buildURL,
			"number": b.id,
		},
	})
}

// handleBuildPoll returns the current build status.
func handleBuildPoll(w http.ResponseWriter, r *http.Request) {
	// Path: /job/.../{buildID}/api/json
	// Strip trailing /api/json, then extract last path segment as build ID.
	trimmed := strings.TrimSuffix(r.URL.Path, "/api/json")
	trimmed = strings.TrimRight(trimmed, "/")
	lastSlash := strings.LastIndex(trimmed, "/")
	if lastSlash < 0 {
		http.NotFound(w, r)
		return
	}
	bID, err := strconv.ParseInt(trimmed[lastSlash+1:], 10, 64)
	if err != nil {
		http.Error(w, "invalid build id in path", http.StatusBadRequest)
		return
	}

	mu.Lock()
	b, ok := builds[bID]
	mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if time.Now().Before(b.startedAt.Add(buildDuration)) {
		// Build is still running
		elapsed := time.Since(b.startedAt)
		if elapsed < 0 {
			elapsed = 0
		}
		log.Printf("  build %d: running (%s elapsed)", bID, elapsed.Round(time.Second))
		json.NewEncoder(w).Encode(map[string]any{
			"building": true,
			"result":   nil,
		})
		return
	}

	// Build is done
	log.Printf("  build %d: complete → %s", bID, buildResult)
	json.NewEncoder(w).Encode(map[string]any{
		"building": false,
		"result":   buildResult,
	})
}
