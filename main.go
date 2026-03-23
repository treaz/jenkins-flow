package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/treaz/jenkins-flow/pkg/logger"
	"github.com/treaz/jenkins-flow/pkg/server"
)

// App is the Wails application struct.
type App struct {
	ctx context.Context
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func main() {
	instancesPath, workflowDirs := resolveConfigPaths()

	l := logger.New(logger.Info)
	srv := server.NewServer(0, instancesPath, workflowDirs, "", l)
	router := srv.BuildRouter()

	// Get the static subdirectory from embedded files (strip "static/" prefix)
	staticFS, err := fs.Sub(server.StaticFiles, "static")
	if err != nil {
		log.Fatalf("Failed to load embedded static files: %v", err)
	}

	app := &App{}

	err = wails.Run(&options.App{
		Title:    "Jenkins Flow",
		Width:    1280,
		Height:   800,
		MinWidth: 800,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets:  staticFS,
			Handler: router,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarDefault(),
			Appearance:           mac.DefaultAppearance,
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		OnStartup: app.startup,
		Bind:      []interface{}{app},
	})
	if err != nil {
		log.Fatalf("Error starting application: %v", err)
	}
}

// resolveConfigPaths resolves instances.yaml and workflow directories.
// When running as a .app bundle, paths are resolved relative to
// ~/.config/jenkins-flow/. When running from a development directory,
// paths are resolved relative to the current working directory.
func resolveConfigPaths() (string, []string) {
	// Check if instances.yaml exists in the current directory (dev mode)
	if _, err := os.Stat("instances.yaml"); err == nil {
		return "instances.yaml", []string{"workflows", "examples"}
	}

	// Fall back to config directory for .app bundle mode
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Warning: could not determine home directory: %v", err)
		return "instances.yaml", []string{"workflows"}
	}

	configDir := filepath.Join(home, ".config", "jenkins-flow")

	instancesPath := filepath.Join(configDir, "instances.yaml")
	workflowsDir := filepath.Join(configDir, "workflows")

	// Create config directory if it doesn't exist
	os.MkdirAll(configDir, 0755)
	os.MkdirAll(workflowsDir, 0755)

	// Check for JENKINS_FLOW_WORKFLOWS env var for additional workflow dirs
	extraDirs := os.Getenv("JENKINS_FLOW_WORKFLOWS")
	dirs := []string{workflowsDir}
	if extraDirs != "" {
		for _, d := range strings.Split(extraDirs, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				dirs = append(dirs, d)
			}
		}
	}

	return instancesPath, dirs
}
