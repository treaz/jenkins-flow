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

// ParallelGroup represents a group of steps to run concurrently.
// All steps must succeed before the workflow proceeds.
type ParallelGroup struct {
	Name  string `yaml:"name,omitempty"` // Optional group name for logging
	Steps []Step `yaml:"steps"`
}

// WorkflowItem represents either a single step or a parallel group.
// Exactly one of Step or Parallel should be populated.
type WorkflowItem struct {
	// Inline step fields (when not using parallel)
	Name     string            `yaml:"name,omitempty"`
	Instance string            `yaml:"instance,omitempty"`
	Job      string            `yaml:"job,omitempty"`
	Params   map[string]string `yaml:"params,omitempty"`
	// Parallel group
	Parallel *ParallelGroup `yaml:"parallel,omitempty"`
}

// IsParallel returns true if this item is a parallel group.
func (w *WorkflowItem) IsParallel() bool {
	return w.Parallel != nil
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
		if item.IsParallel() {
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
