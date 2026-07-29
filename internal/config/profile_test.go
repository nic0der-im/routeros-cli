package config

import "testing"

func TestResolveProfile(t *testing.T) {
	tests := []struct {
		name     string
		explicit string
		readOnly bool
		want     string
		wantErr  bool
	}{
		{"default operator", "", false, ProfileOperator, false},
		{"default agent from read-only", "", true, ProfileAgent, false},
		{"explicit operator", "operator", true, ProfileOperator, false},
		{"explicit agent", "agent", false, ProfileAgent, false},
		{"explicit agent-strict", "agent-strict", false, ProfileAgentStrict, false},
		{"case normalize", "Agent-Strict", false, ProfileAgentStrict, false},
		{"invalid", "god-mode", false, "", true},
		{"whitespace", "  agent  ", false, ProfileAgent, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveProfile(tt.explicit, tt.readOnly)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestResolveProfileFromEnv(t *testing.T) {
	t.Setenv("ROS_PROFILE", "")
	t.Setenv("ROS_READ_ONLY", "")
	got, err := ResolveProfileFromEnv()
	if err != nil || got != ProfileOperator {
		t.Fatalf("unset: got %q err %v", got, err)
	}

	t.Setenv("ROS_READ_ONLY", "1")
	got, err = ResolveProfileFromEnv()
	if err != nil || got != ProfileAgent {
		t.Fatalf("read-only default: got %q err %v", got, err)
	}

	t.Setenv("ROS_PROFILE", "operator")
	got, err = ResolveProfileFromEnv()
	if err != nil || got != ProfileOperator {
		t.Fatalf("explicit wins: got %q err %v", got, err)
	}

	t.Setenv("ROS_PROFILE", "agent-strict")
	t.Setenv("ROS_READ_ONLY", "")
	got, err = ResolveProfileFromEnv()
	if err != nil || got != ProfileAgentStrict {
		t.Fatalf("agent-strict: got %q err %v", got, err)
	}

	t.Setenv("ROS_PROFILE", "nope")
	if _, err := ResolveProfileFromEnv(); err == nil {
		t.Fatal("expected invalid profile error")
	}
}

func TestProfileHelpers(t *testing.T) {
	if ProfileForcesStrict(ProfileOperator) || ProfileForcesStrict(ProfileAgent) {
		t.Fatal("operator/agent should not force strict")
	}
	if !ProfileForcesStrict(ProfileAgentStrict) {
		t.Fatal("agent-strict should force strict")
	}
	if ProfileRequiresSafeSession(ProfileOperator) {
		t.Fatal("operator should not require safe session by profile")
	}
	if !ProfileRequiresSafeSession(ProfileAgent) || !ProfileRequiresSafeSession(ProfileAgentStrict) {
		t.Fatal("agent profiles require safe session")
	}

	t.Setenv("ROS_STRICT", "")
	if EffectiveStrict(ProfileOperator) {
		t.Fatal("operator without ROS_STRICT")
	}
	if !EffectiveStrict(ProfileAgentStrict) {
		t.Fatal("agent-strict implies strict")
	}
	t.Setenv("ROS_STRICT", "1")
	if !EffectiveStrict(ProfileOperator) {
		t.Fatal("ROS_STRICT should force strict for any profile")
	}
}
