package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestThinkingSourceReasoningFirst(t *testing.T) {
	var source thinkingStreamSource

	if !allowReasoningSource(&source) {
		t.Fatalf("expected reasoning source to be accepted first")
	}
	if source != thinkingSourceReasoningEvent {
		t.Fatalf("expected source to be reasoning, got %v", source)
	}
	if allowTagSource(&source) {
		t.Fatalf("expected tag source to be rejected after reasoning source selected")
	}
}

func TestThinkingSourceTagFirst(t *testing.T) {
	var source thinkingStreamSource

	if !allowTagSource(&source) {
		t.Fatalf("expected tag source to be accepted first")
	}
	if source != thinkingSourceTagBlock {
		t.Fatalf("expected source to be tag, got %v", source)
	}
	if allowReasoningSource(&source) {
		t.Fatalf("expected reasoning source to be rejected after tag source selected")
	}
}

func TestThinkingSourceSameSourceRemainsAllowed(t *testing.T) {
	var source thinkingStreamSource

	if !allowTagSource(&source) {
		t.Fatalf("expected initial tag source selection to succeed")
	}
	if !allowTagSource(&source) {
		t.Fatalf("expected repeated tag source selection to stay allowed")
	}

	source = thinkingSourceUnknown
	if !allowReasoningSource(&source) {
		t.Fatalf("expected initial reasoning source selection to succeed")
	}
	if !allowReasoningSource(&source) {
		t.Fatalf("expected repeated reasoning source selection to stay allowed")
	}
}

func TestValidateOpenAIRequestShapeRejectsAssistantPrefill(t *testing.T) {
	req := &OpenAIRequest{
		Messages: []OpenAIMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "prefill"},
		},
	}

	if msg := validateOpenAIRequestShape(req); msg == "" {
		t.Fatalf("expected assistant-prefill final message to be rejected")
	}
}

func TestValidateOpenAIRequestShapeAllowsToolResultFinalTurn(t *testing.T) {
	req := &OpenAIRequest{
		Messages: []OpenAIMessage{
			{Role: "user", Content: "find weather"},
			{
				Role: "assistant",
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: "get_weather", Arguments: "{}"},
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: "sunny"},
		},
	}

	if msg := validateOpenAIRequestShape(req); msg != "" {
		t.Fatalf("expected tool-result final turn to be valid, got %q", msg)
	}
}

func TestValidateClaudeRequestShapeRejectsAssistantPrefill(t *testing.T) {
	req := &ClaudeRequest{
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "prefill"},
		},
	}

	if msg := validateClaudeRequestShape(req); msg == "" {
		t.Fatalf("expected assistant-prefill final message to be rejected")
	}
}

func TestMergeUniqueModelsPreservesUnionAcrossAccounts(t *testing.T) {
	base := []ModelInfo{
		{ModelId: "claude-sonnet-4.5", InputTypes: []string{"TEXT"}},
	}
	incoming := []ModelInfo{
		{ModelId: "claude-sonnet-4.5", InputTypes: []string{"image"}},
		{ModelId: "claude-opus-4-7", InputTypes: []string{"text"}},
	}

	merged := mergeUniqueModels(base, incoming)
	if len(merged) != 2 {
		t.Fatalf("expected 2 unique models, got %d", len(merged))
	}
	if !modelSupportsImage(merged[0].InputTypes) {
		t.Fatalf("expected merged input types to preserve image capability, got %#v", merged[0].InputTypes)
	}
	if merged[1].ModelId != "claude-opus-4-7" {
		t.Fatalf("expected second model to be claude-opus-4-7, got %q", merged[1].ModelId)
	}
}

func TestBuildAnthropicModelsResponseGeneratesThinkingVariants(t *testing.T) {
	models := buildAnthropicModelsResponse([]ModelInfo{{
		ModelId:    "claude-sonnet-4.5",
		InputTypes: []string{"text", "image"},
	}}, "-thinking")

	if len(models) != 2 {
		t.Fatalf("expected base model and thinking variant, got %d", len(models))
	}
	if models[0]["id"] != "claude-sonnet-4.5" {
		t.Fatalf("unexpected base model id: %#v", models[0]["id"])
	}
	if models[1]["id"] != "claude-sonnet-4.5-thinking" {
		t.Fatalf("unexpected thinking model id: %#v", models[1]["id"])
	}
	if supportsImage, ok := models[0]["supports_image"].(bool); !ok || !supportsImage {
		t.Fatalf("expected image capability to be preserved, got %#v", models[0]["supports_image"])
	}
}

func TestHandleModelRetrieveOpenAIShape(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4o", nil)
	rec := httptest.NewRecorder()

	h.handleModelRetrieve(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["id"] != "gpt-4o" || body["object"] != "model" {
		t.Fatalf("expected OpenAI model object, got %#v", body)
	}
}

func TestCanonicalAPIPathToleratesEndpointBaseURLMisconfiguration(t *testing.T) {
	cases := map[string]string{
		"/v1/v1/responses":                   "/v1/responses",
		"/v1/responses/responses":            "/v1/responses",
		"/v1/chat/completions/responses":     "/v1/responses",
		"/v1/chat/completions":               "/v1/chat/completions",
		"/openai/v1/chat/completions":        "/v1/chat/completions",
		"/openai/v1/models/gpt-4o":           "/v1/models/gpt-4o",
		"/v1/chat/completions/models/gpt-4o": "/v1/models/gpt-4o",
		"/openai/v1/messages?ignored=false":  "/openai/v1/messages?ignored=false",
		"/openai/v1/messages":                "/v1/messages",
		"/admin/api/settings":                "/admin/api/settings",
	}

	for input, want := range cases {
		if got := canonicalAPIPath(input); got != want {
			t.Fatalf("canonicalAPIPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestServeHTTPToleratesNestedModelRetrievePath(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/openai/v1/models/gpt-4o", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected nested model path to return 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if body["id"] != "gpt-4o" {
		t.Fatalf("expected model id gpt-4o, got %#v", body["id"])
	}
}

func TestRequestLogCapturesNormalizedPathAndErrorMessage(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/responses", nil)
	body := `{"error":{"type":"invalid_request_error","message":"route missing"}}`

	h.recordRequestLog(req, "/v1/chat/completions/responses", "/v1/responses", http.StatusNotFound, 12*time.Millisecond, body)
	logs, total := h.queryRequestLogs(10, "warn", "responses")

	if total != 1 {
		t.Fatalf("expected total log count 1, got %d", total)
	}
	if len(logs) != 1 {
		t.Fatalf("expected one filtered log, got %d", len(logs))
	}
	if logs[0].CanonicalPath != "/v1/responses" {
		t.Fatalf("expected canonical path, got %q", logs[0].CanonicalPath)
	}
	if !strings.Contains(logs[0].Message, "route missing") {
		t.Fatalf("expected extracted error message, got %q", logs[0].Message)
	}
}

func TestRequestLogClear(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	h.recordRequestLog(req, "/v1/models", "/v1/models", http.StatusOK, time.Millisecond, "")
	h.clearRequestLogs()
	logs, total := h.queryRequestLogs(10, "", "")

	if total != 0 || len(logs) != 0 {
		t.Fatalf("expected logs to be cleared, total=%d len=%d", total, len(logs))
	}
}

func TestLoggingResponseWriterCapturesWithoutTruncatingResponse(t *testing.T) {
	rec := httptest.NewRecorder()
	writer := &loggingResponseWriter{ResponseWriter: rec}
	payload := strings.Repeat("x", requestLogBodyLimit+128)

	n, err := writer.Write([]byte(payload))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("expected full payload write count %d, got %d", len(payload), n)
	}
	if rec.Body.String() != payload {
		t.Fatalf("expected downstream response to remain untruncated")
	}
	if writer.body.Len() != requestLogBodyLimit {
		t.Fatalf("expected captured log body to be capped at %d, got %d", requestLogBodyLimit, writer.body.Len())
	}
}
