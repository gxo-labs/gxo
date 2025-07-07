package config

import "time"

// StateAccessMode defines the available methods for accessing state data.
// It is a typed string to enforce valid values.
type StateAccessMode string

const (
	// StateAccessDeepCopy (default) ensures that any data read from the state store
	// is a deep copy, preventing any possibility of a module mutating the shared state.
	// This is the safest and recommended mode.
	StateAccessDeepCopy StateAccessMode = "deep_copy"

	// StateAccessUnsafeDirectReference provides a direct reference to the data in the state
	// store. This offers the highest performance but carries the risk that a module
	// could mutate a shared map or slice. This should only be used for trusted,
	// performance-critical tasks where the developer guarantees no mutation occurs.
	StateAccessUnsafeDirectReference StateAccessMode = "unsafe_direct_reference"
)

// StatePolicy defines the rules for how tasks interact with the state store.
// It can be defined globally at the playbook level and overridden per-task.
type StatePolicy struct {
	// AccessMode controls the method used for reading from the state store.
	// Valid values are "deep_copy" or "unsafe_direct_reference".
	// If unset, it defaults to "deep_copy" for maximum safety.
	AccessMode StateAccessMode `yaml:"access_mode,omitempty" json:"access_mode,omitempty"`
}

// StallPolicy defines the parameters for the engine's stall detection mechanism.
// This policy is configured programmatically and not via playbook YAML.
type StallPolicy struct {
	// Interval is the frequency at which the engine checks for progress.
	Interval time.Duration `yaml:"-" json:"-"`
	// Tolerance is the number of consecutive check intervals with no progress
	// before the engine declares the playbook as stalled and halts execution.
	Tolerance int `yaml:"-" json:"-"`
}