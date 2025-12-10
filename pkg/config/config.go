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

type Config struct {
	Instances map[string]Instance `yaml:"instances"`
	Workflow  []Step              `yaml:"workflow"`
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
		Workflow []Step `yaml:"workflow"`
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

	for i, step := range c.Workflow {
		if step.Instance == "" {
			return fmt.Errorf("step %d (%q) missing instance", i, step.Name)
		}
		if _, ok := c.Instances[step.Instance]; !ok {
			return fmt.Errorf("step %d (%q) refers to unknown instance %q", i, step.Name, step.Instance)
		}
		if step.Job == "" {
			return fmt.Errorf("step %d (%q) missing job path", i, step.Name)
		}
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
