package config

import (
	"fmt"
	"os"
	"strings"
)

// Runtime profile constants (ROS_PROFILE).
const (
	ProfileOperator    = "operator"
	ProfileAgent       = "agent"
	ProfileAgentStrict = "agent-strict"
)

// ResolveProfileFromEnv returns the active runtime profile.
// ROS_PROFILE=operator|agent|agent-strict wins when set.
// When unset: ROS_READ_ONLY=1/true → agent, else operator.
func ResolveProfileFromEnv() (string, error) {
	return ResolveProfile(os.Getenv("ROS_PROFILE"), envTruthy("ROS_READ_ONLY"))
}

// ResolveProfile resolves a profile from an explicit value and read-only hint.
// Explicit empty profile falls back to agent when readOnlyDefault is true.
func ResolveProfile(explicit string, readOnlyDefault bool) (string, error) {
	v := strings.ToLower(strings.TrimSpace(explicit))
	if v == "" {
		if readOnlyDefault {
			return ProfileAgent, nil
		}
		return ProfileOperator, nil
	}
	switch v {
	case ProfileOperator, ProfileAgent, ProfileAgentStrict:
		return v, nil
	default:
		return "", fmt.Errorf("invalid ROS_PROFILE %q (valid: %s, %s, %s)",
			explicit, ProfileOperator, ProfileAgent, ProfileAgentStrict)
	}
}

// ProfileForcesStrict reports whether the profile forces ROS_STRICT semantics
// (all devices treated as prod for guardrails).
func ProfileForcesStrict(profile string) bool {
	return strings.EqualFold(strings.TrimSpace(profile), ProfileAgentStrict)
}

// ProfileRequiresSafeSession reports whether the profile requires an active
// safe session before writes on every device (even lab).
func ProfileRequiresSafeSession(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case ProfileAgent, ProfileAgentStrict:
		return true
	default:
		return false
	}
}

// EffectiveStrict combines ROS_STRICT env with agent-strict profile.
func EffectiveStrict(profile string) bool {
	return ROSStrict() || ProfileForcesStrict(profile)
}

func envTruthy(key string) bool {
	v := os.Getenv(key)
	return v == "1" || strings.EqualFold(v, "true")
}
