package proxy

import (
	"encoding/json"
	"kiro-go/config"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func initSettingsTestConfig(t *testing.T) {
	t.Helper()

	cfg := config.Config{
		Password:                   "changeme",
		Port:                       8080,
		Host:                       "127.0.0.1",
		ApiKey:                     "sk-existing",
		RequireApiKey:              true,
		RestrictProAccountsAtLimit: true,
		Accounts:                   []config.Account{},
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

func TestAPIGetSettingsIncludesProLimitRestriction(t *testing.T) {
	initSettingsTestConfig(t)
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/settings", nil)
	rec := httptest.NewRecorder()

	h.apiGetSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["restrictProAccountsAtLimit"] != true {
		t.Fatalf("expected restrictProAccountsAtLimit in settings response, got %#v", body["restrictProAccountsAtLimit"])
	}
}

func TestAPIUpdateSettingsTogglesProLimitRestriction(t *testing.T) {
	initSettingsTestConfig(t)
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/settings", strings.NewReader(`{"restrictProAccountsAtLimit":false}`))
	rec := httptest.NewRecorder()

	h.apiUpdateSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", rec.Code)
	}
	if config.IsRestrictProAccountsAtLimit() {
		t.Fatalf("expected Pro limit restriction to be disabled")
	}
}

func TestAPIUpdateSettingsPreservesOmittedFields(t *testing.T) {
	initSettingsTestConfig(t)
	h := &Handler{}

	req := httptest.NewRequest(http.MethodPost, "/admin/api/settings", strings.NewReader(`{"password":"new-password"}`))
	rec := httptest.NewRecorder()

	h.apiUpdateSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", rec.Code)
	}
	if config.GetApiKey() != "sk-existing" {
		t.Fatalf("expected omitted API key to be preserved")
	}
	if !config.IsApiKeyRequired() {
		t.Fatalf("expected omitted API key requirement to be preserved")
	}
	if !config.IsRestrictProAccountsAtLimit() {
		t.Fatalf("expected omitted Pro limit restriction to be preserved")
	}
}
