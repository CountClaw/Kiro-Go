package pool

import (
	"encoding/json"
	"kiro-go/config"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writePoolTestConfig(t *testing.T, restrictProAccountsAtLimit bool) {
	t.Helper()

	cfg := config.Config{
		Password:                   "changeme",
		Port:                       8080,
		Host:                       "127.0.0.1",
		RequireApiKey:              false,
		RestrictProAccountsAtLimit: restrictProAccountsAtLimit,
		Accounts: []config.Account{{
			ID:               "pro-limited",
			AccessToken:      "token",
			Region:           "us-east-1",
			Enabled:          true,
			SubscriptionType: "PRO",
			UsageCurrent:     100,
			UsageLimit:       100,
		}},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config failed: %v", err)
	}

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	if err := config.Init(path); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
}

func TestGetNextSkipsLimitedProAccountWhenRestrictionEnabled(t *testing.T) {
	writePoolTestConfig(t, true)
	p := &AccountPool{
		cooldowns:   make(map[string]time.Time),
		errorCounts: make(map[string]int),
	}
	p.Reload()

	if got := p.GetNext(); got != nil {
		t.Fatalf("expected limited Pro account to be skipped, got %q", got.ID)
	}
	if count := p.AvailableCount(); count != 0 {
		t.Fatalf("expected no available accounts, got %d", count)
	}
}

func TestGetNextAllowsLimitedProAccountWhenRestrictionDisabled(t *testing.T) {
	writePoolTestConfig(t, false)
	p := &AccountPool{
		cooldowns:   make(map[string]time.Time),
		errorCounts: make(map[string]int),
	}
	p.Reload()

	got := p.GetNext()
	if got == nil {
		t.Fatalf("expected limited Pro account to be allowed")
	}
	if got.ID != "pro-limited" {
		t.Fatalf("expected pro-limited account, got %q", got.ID)
	}
	if count := p.AvailableCount(); count != 1 {
		t.Fatalf("expected one available account, got %d", count)
	}
}
