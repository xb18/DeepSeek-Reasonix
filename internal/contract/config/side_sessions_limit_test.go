package config

import (
	"slices"
	"testing"
)

func TestSideSessionLimitIsUnlimitedUnlessTheUserSetsOne(t *testing.T) {
	for in, want := range map[int]int{-4: 0, 0: 0, 3: 3, 40: 40} {
		cfg := &Config{}
		cfg.Agent.MaxSideSessions = in
		if got := cfg.SideSessionLimit(); got != want {
			t.Errorf("max_side_sessions %d -> %d, want %d", in, got, want)
		}
	}
	if Default().SideSessionLimit() != 0 {
		t.Error("the default limits side conversations")
	}
	var none *Config
	if none.SideSessionLimit() != 0 {
		t.Error("a nil config limits side conversations")
	}
}

func TestProjectCannotSetTheSideSessionLimit(t *testing.T) {
	cfg, _ := loadScoped(t, "", "[agent]\nmax_side_sessions = 1\n")
	if cfg.SideSessionLimit() != 0 {
		t.Fatal("a cloned repo's reasonix.toml limited the user's side conversations")
	}
	if !slices.Contains(ignoredKeys(cfg), "agent.max_side_sessions") {
		t.Fatalf("ignored = %v, want agent.max_side_sessions reported", ignoredKeys(cfg))
	}
	cfg, _ = loadScoped(t, "[agent]\nmax_side_sessions = 5\n", "[agent]\nmax_side_sessions = 1\n")
	if cfg.SideSessionLimit() != 5 {
		t.Fatalf("the project file overrode the user's limit: %d", cfg.SideSessionLimit())
	}
}

func TestQueuedInputLimitIsUnlimitedUnlessTheUserSetsOne(t *testing.T) {
	for in, want := range map[int]int{-4: 0, 0: 0, 64: 64, 500: 500} {
		cfg := &Config{}
		cfg.Agent.MaxQueuedInputs = in
		if got := cfg.QueuedInputLimit(); got != want {
			t.Errorf("max_queued_inputs %d -> %d, want %d", in, got, want)
		}
	}
	if Default().QueuedInputLimit() != 0 {
		t.Error("the default limits the queue")
	}
	var none *Config
	if none.QueuedInputLimit() != 0 {
		t.Error("a nil config limits the queue")
	}
}

func TestProjectCannotSetTheQueuedInputLimit(t *testing.T) {
	cfg, _ := loadScoped(t, "", "[agent]\nmax_queued_inputs = 1\n")
	if cfg.QueuedInputLimit() != 0 {
		t.Fatal("a cloned repo's reasonix.toml limited the user's queue")
	}
	if !slices.Contains(ignoredKeys(cfg), "agent.max_queued_inputs") {
		t.Fatalf("ignored = %v, want agent.max_queued_inputs reported", ignoredKeys(cfg))
	}
	cfg, _ = loadScoped(t, "[agent]\nmax_queued_inputs = 80\n", "[agent]\nmax_queued_inputs = 1\n")
	if cfg.QueuedInputLimit() != 80 {
		t.Fatalf("the project file overrode the user's limit: %d", cfg.QueuedInputLimit())
	}
}

func TestProviderRetriesDefaultToTenAndZeroMeansNoLimit(t *testing.T) {
	if got := Default().Agent.MaxProviderRetries; got != 10 {
		t.Fatalf("default max_provider_retries = %d, want 10", got)
	}
	cfg, _ := loadScoped(t, "[agent]\nmax_provider_retries = 0\n", "")
	if cfg.Agent.MaxProviderRetries != 0 {
		t.Fatalf("an explicit 0 read as %d", cfg.Agent.MaxProviderRetries)
	}
	cfg, _ = loadScoped(t, "", "")
	if cfg.Agent.MaxProviderRetries != 10 {
		t.Fatalf("an absent key read as %d, want the default 10", cfg.Agent.MaxProviderRetries)
	}
}

func TestProjectCannotSetTheProviderRetryLimit(t *testing.T) {
	cfg, _ := loadScoped(t, "", "[agent]\nmax_provider_retries = 0\n")
	if cfg.Agent.MaxProviderRetries != 10 {
		t.Fatal("a cloned repo's reasonix.toml removed the user's retry limit")
	}
	if !slices.Contains(ignoredKeys(cfg), "agent.max_provider_retries") {
		t.Fatalf("ignored = %v, want agent.max_provider_retries reported", ignoredKeys(cfg))
	}
	cfg, _ = loadScoped(t, "[agent]\nmax_provider_retries = 25\n", "[agent]\nmax_provider_retries = 1\n")
	if cfg.Agent.MaxProviderRetries != 25 {
		t.Fatalf("the project file overrode the user's limit: %d", cfg.Agent.MaxProviderRetries)
	}
}

func TestSubagentGuidanceLimitsDefaultToANamedBoundAndZeroLiftsThem(t *testing.T) {
	if b, m := Default().SubagentSteerLimits(); b != 16384 || m != 30 {
		t.Fatalf("defaults = %d bytes, %d per minute, want 16384 and 30", b, m)
	}
	cfg, _ := loadScoped(t, "[agent]\nmax_subagent_steer_bytes = 0\nmax_subagent_steers_per_minute = 0\n", "")
	if b, m := cfg.SubagentSteerLimits(); b != 0 || m != 0 {
		t.Fatalf("explicit zeros read as %d and %d, want both lifted", b, m)
	}
	cfg, _ = loadScoped(t, "[agent]\nmax_subagent_steer_bytes = -5\n", "")
	if b, _ := cfg.SubagentSteerLimits(); b != 0 {
		t.Fatalf("a negative value read as %d, want lifted", b)
	}
	cfg, _ = loadScoped(t, "", "")
	if b, m := cfg.SubagentSteerLimits(); b != 16384 || m != 30 {
		t.Fatalf("absent keys read as %d and %d, want the defaults", b, m)
	}
}

func TestProjectCannotLoosenTheSubagentGuidanceLimits(t *testing.T) {
	cfg, _ := loadScoped(t, "", "[agent]\nmax_subagent_steer_bytes = 0\nmax_subagent_steers_per_minute = 0\n")
	if b, m := cfg.SubagentSteerLimits(); b != 16384 || m != 30 {
		t.Fatalf("a cloned repo's reasonix.toml moved the limits to %d and %d", b, m)
	}
	for _, key := range []string{"agent.max_subagent_steer_bytes", "agent.max_subagent_steers_per_minute"} {
		if !slices.Contains(ignoredKeys(cfg), key) {
			t.Fatalf("ignored = %v, want %s reported", ignoredKeys(cfg), key)
		}
	}
	cfg, _ = loadScoped(t, "[agent]\nmax_subagent_steer_bytes = 4096\n", "[agent]\nmax_subagent_steer_bytes = 1\n")
	if b, _ := cfg.SubagentSteerLimits(); b != 4096 {
		t.Fatalf("the project file overrode the user's limit: %d", b)
	}
}
