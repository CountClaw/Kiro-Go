package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestrictProAccountsAtLimitDefaultsToEnabledForNewConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := Init(path); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !IsRestrictProAccountsAtLimit() {
		t.Fatalf("expected Pro limit restriction to default to enabled")
	}
}

func TestRestrictProAccountsAtLimitDefaultsToEnabledForExistingConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := []byte(`{"password":"changeme","port":8080,"host":"0.0.0.0","requireApiKey":false,"accounts":[]}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if err := Init(path); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !IsRestrictProAccountsAtLimit() {
		t.Fatalf("expected missing Pro limit restriction field to default to enabled")
	}
}

func TestRestrictProAccountsAtLimitAllowsExplicitFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := []byte(`{"password":"changeme","port":8080,"host":"0.0.0.0","requireApiKey":false,"restrictProAccountsAtLimit":false,"accounts":[]}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	if err := Init(path); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if IsRestrictProAccountsAtLimit() {
		t.Fatalf("expected explicit false Pro limit restriction setting to be preserved")
	}
}

func TestShouldRestrictAccountAtLimitHonorsProSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	content := []byte(`{"password":"changeme","port":8080,"host":"0.0.0.0","requireApiKey":false,"restrictProAccountsAtLimit":false,"accounts":[]}`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	if err := Init(path); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	proAccount := Account{SubscriptionType: "PRO", UsageCurrent: 100, UsageLimit: 100}
	if ShouldRestrictAccountAtLimit(proAccount) {
		t.Fatalf("expected limited Pro account to be allowed when Pro restriction is disabled")
	}

	freeAccount := Account{SubscriptionType: "FREE", UsageCurrent: 100, UsageLimit: 100}
	if !ShouldRestrictAccountAtLimit(freeAccount) {
		t.Fatalf("expected limited non-Pro account to remain restricted")
	}
}
