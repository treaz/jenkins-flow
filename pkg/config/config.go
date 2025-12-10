package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Instance struct {
	URL     string `yaml:"url"`
	AuthEnv string `yaml:"auth_env,omitempty"`
	Token   string `yaml:"token,omitempty"` // Direct token storage
}

type Step struct {
	Name     string            `yaml:"name"`
	Instance string            `yaml:"instance"`
	Job      string            `yaml:"job"`
	Params   map[string]string `yaml:"params,omitempty"` // Job parameters
}

// GitHubConfig holds global GitHub authentication settings
type GitHubConfig struct {
	AuthEnv string `yaml:"auth_env,omitempty"` // Env var with GitHub token
	Token   string `yaml:"token,omitempty"`    // Direct token (local only)
}

// GetToken retrieves the GitHub token from env var or direct config
func (g GitHubConfig) GetToken() (string, error) {
	if g.Token != "" {
		return g.Token, nil
	}
	if g.AuthEnv != "" {
		val := os.Getenv(g.AuthEnv)
		if val == "" {
			return "", fmt.Errorf("environment variable %q is not set", g.AuthEnv)
		}
		return val, nil
	}
	// Empty token is valid for public repos
	return "", nil
}

// PRWait represents a wait condition for a GitHub PR
type PRWait struct {
	Name     string `yaml:"name"`
	Owner    string `yaml:"owner"`               // GitHub org/user
	Repo     string `yaml:"repo"`                // Repository name
	PRNumber int    `yaml:"pr_number"`           // PR number to monitor
	WaitFor  string `yaml:"wait_for"`            // Target state: "merged", "closed"
	PollSecs int    `yaml:"poll_secs,omitempty"` // Poll interval (default: 30)
}

// ParallelGroup represents a group of steps to run concurrently.
// All steps must succeed before the workflow proceeds.
type ParallelGroup struct {
	Name  string `yaml:"name,omitempty"` // Optional group name for logging
	Steps []Step `yaml:"steps"`
}

// WorkflowItem represents either a single step, a parallel group, or a PR wait.
// Exactly one of Step, Parallel, or WaitForPR should be populated.
type WorkflowItem struct {
	// Inline step fields (when not using parallel)
	Name     string            `yaml:"name,omitempty"`
	Instance string            `yaml:"instance,omitempty"`
	Job      string            `yaml:"job,omitempty"`
	Params   map[string]string `yaml:"params,omitempty"`
	// Parallel group
	Parallel *ParallelGroup `yaml:"parallel,omitempty"`
	// PR wait (trigger on PR merge/close)
	WaitForPR *PRWait `yaml:"wait_for_pr,omitempty"`
}

// IsParallel returns true if this item is a parallel group.
func (w *WorkflowItem) IsParallel() bool {
	return w.Parallel != nil
}

// IsPRWait returns true if this item is a PR wait condition.
func (w *WorkflowItem) IsPRWait() bool {
	return w.WaitForPR != nil
}

// AsStep converts inline step fields to a Step struct.
func (w *WorkflowItem) AsStep() Step {
	return Step{
		Name:     w.Name,
		Instance: w.Instance,
		Job:      w.Job,
		Params:   w.Params,
	}
}

type Config struct {
	Instances map[string]Instance `yaml:"instances"`
	GitHub    *GitHubConfig       `yaml:"github,omitempty"` // Global GitHub config
	Workflow  []WorkflowItem      `yaml:"workflow"`
}

func Load(instancesPath, workflowPath string) (*Config, error) {
	// 1. Load Instances
	instancesData, err := os.ReadFile(instancesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read instances config (%s): %w", instancesPath, err)
	}

	var instancesCfg struct {
		Instances map[string]Instance `yaml:"instances"`
		GitHub    *GitHubConfig       `yaml:"github,omitempty"`
	}
	if err := yaml.Unmarshal(instancesData, &instancesCfg); err != nil {
		return nil, fmt.Errorf("failed to parse instances config: %w", err)
	}

	// 2. Load Workflow
	workflowData, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow config (%s): %w", workflowPath, err)
	}

	var workflowCfg struct {
		Workflow []WorkflowItem `yaml:"workflow"`
	}
	if err := yaml.Unmarshal(workflowData, &workflowCfg); err != nil {
		return nil, fmt.Errorf("failed to parse workflow config: %w", err)
	}

	// 3. Merge
	cfg := &Config{
		Instances: instancesCfg.Instances,
		GitHub:    instancesCfg.GitHub,
		Workflow:  workflowCfg.Workflow,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if len(c.Instances) == 0 {
		return fmt.Errorf("no instances defined")
	}
	if len(c.Workflow) == 0 {
		return fmt.Errorf("workflow is empty")
	}

	for name, inst := range c.Instances {
		if inst.URL == "" {
			return fmt.Errorf("instance %q has empty URL", name)
		}
		if inst.AuthEnv == "" && inst.Token == "" {
			return fmt.Errorf("instance %q must have either 'auth_env' or 'token' set", name)
		}
	}

	for i, item := range c.Workflow {
		if item.IsPRWait() {
			// Validate PR wait
			if err := c.validatePRWait(item.WaitForPR, fmt.Sprintf("wait_for_pr[%d]", i)); err != nil {
				return err
			}
		} else if item.IsParallel() {
			// Validate parallel group
			if len(item.Parallel.Steps) == 0 {
				return fmt.Errorf("workflow item %d: parallel group is empty", i)
			}
			for j, step := range item.Parallel.Steps {
				if err := c.validateStep(step, fmt.Sprintf("parallel[%d].step[%d]", i, j)); err != nil {
					return err
				}
			}
		} else {
			// Validate single step
			step := item.AsStep()
			if err := c.validateStep(step, fmt.Sprintf("step %d", i)); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateStep validates a single step configuration.
func (c *Config) validateStep(step Step, location string) error {
	if step.Name == "" {
		return fmt.Errorf("%s: missing name", location)
	}
	if step.Instance == "" {
		return fmt.Errorf("%s (%q): missing instance", location, step.Name)
	}
	if _, ok := c.Instances[step.Instance]; !ok {
		return fmt.Errorf("%s (%q): unknown instance %q", location, step.Name, step.Instance)
	}
	if step.Job == "" {
		return fmt.Errorf("%s (%q): missing job path", location, step.Name)
	}
	return nil
}

// validatePRWait validates a PR wait configuration.
func (c *Config) validatePRWait(pr *PRWait, location string) error {
	if pr.Name == "" {
		return fmt.Errorf("%s: missing name", location)
	}
	if pr.Owner == "" {
		return fmt.Errorf("%s (%q): missing owner", location, pr.Name)
	}
	if pr.Repo == "" {
		return fmt.Errorf("%s (%q): missing repo", location, pr.Name)
	}
	if pr.PRNumber <= 0 {
		return fmt.Errorf("%s (%q): invalid pr_number", location, pr.Name)
	}
	if pr.WaitFor == "" {
		return fmt.Errorf("%s (%q): missing wait_for", location, pr.Name)
	}
	if pr.WaitFor != "merged" && pr.WaitFor != "closed" {
		return fmt.Errorf("%s (%q): wait_for must be 'merged' or 'closed', got %q", location, pr.Name, pr.WaitFor)
	}
	return nil
}

func (i Instance) GetToken() (string, error) {
	if i.Token != "" {
		return i.Token, nil
	}
	val := os.Getenv(i.AuthEnv)
	if val == "" {
		return "", fmt.Errorf("environment variable %q is not set", i.AuthEnv)
	}
	return val, nil
}
