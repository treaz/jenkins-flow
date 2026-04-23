package workflow

import (
	"fmt"
	"sync"
)

// Outputs is a thread-safe store of per-step outputs (build_number, build_url, ...)
// surfaced to substitution as ${steps.<id>.<field>}.
type Outputs struct {
	mu sync.RWMutex
	m  map[string]map[string]string
}

// NewOutputs creates an empty Outputs store.
func NewOutputs() *Outputs {
	return &Outputs{m: map[string]map[string]string{}}
}

// Set records a single field for a step ID.
func (o *Outputs) Set(stepID, field, value string) {
	if stepID == "" || field == "" {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.m[stepID] == nil {
		o.m[stepID] = map[string]string{}
	}
	o.m[stepID][field] = value
}

// Get returns a field for a step ID, with an ok flag.
func (o *Outputs) Get(stepID, field string) (string, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if fields, ok := o.m[stepID]; ok {
		v, ok := fields[field]
		return v, ok
	}
	return "", false
}

// Flat returns a snapshot keyed as "steps.<id>.<field>" -> value, suitable
// for merging with cfg.Inputs and passing to config.Substitute.
func (o *Outputs) Flat() map[string]string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string]string, len(o.m)*2)
	for id, fields := range o.m {
		for field, value := range fields {
			out[fmt.Sprintf("steps.%s.%s", id, field)] = value
		}
	}
	return out
}
