package proxy

import (
	"strings"
	"testing"
)

func TestExtractOpenAIMessageTextStructured(t *testing.T) {
	content := []interface{}{
		map[string]interface{}{"type": "text", "text": "alpha"},
		map[string]interface{}{"type": "input_text", "text": "beta"},
	}

	if got := extractOpenAIMessageText(content); got != "alphabeta" {
		t.Fatalf("expected concatenated structured text, got %q", got)
	}

	nested := map[string]interface{}{
		"content": []interface{}{map[string]interface{}{"type": "text", "text": "nested"}},
	}
	if got := extractOpenAIMessageText(nested); got != "nested" {
		t.Fatalf("expected nested content extraction, got %q", got)
	}
}

func TestOpenAIToKiroPreservesStructuredAssistantAndToolContent(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{
				Role: "system",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "system-a"},
					map[string]interface{}{"type": "text", "text": "system-b"},
				},
			},
			{Role: "user", Content: "first-question"},
			{
				Role: "assistant",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "assistant-structured"},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call_1",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "tool-result-structured"},
				},
			},
		},
	}

	payload := OpenAIToKiro(req, false)

	if len(payload.ConversationState.History) != 2 {
		t.Fatalf("expected 2 history items, got %d", len(payload.ConversationState.History))
	}

	firstHistoryUser := payload.ConversationState.History[0].UserInputMessage
	if firstHistoryUser == nil {
		t.Fatalf("expected first history item to be user message")
	}
	if !strings.Contains(firstHistoryUser.Content, "system-a") ||
		!strings.Contains(firstHistoryUser.Content, "system-b") ||
		!strings.Contains(firstHistoryUser.Content, "first-question") {
		t.Fatalf("expected merged system+user content, got %q", firstHistoryUser.Content)
	}

	historyAssistant := payload.ConversationState.History[1].AssistantResponseMessage
	if historyAssistant == nil {
		t.Fatalf("expected second history item to be assistant message")
	}
	if historyAssistant.Content != "assistant-structured" {
		t.Fatalf("expected assistant structured content to be preserved, got %q", historyAssistant.Content)
	}

	cur := payload.ConversationState.CurrentMessage.UserInputMessage
	if !strings.Contains(cur.Content, "tool-result-structured") {
		t.Fatalf("expected tool-result continuation content, got %q", cur.Content)
	}
	if cur.UserInputMessageContext == nil || len(cur.UserInputMessageContext.ToolResults) != 1 {
		t.Fatalf("expected one tool result in current context")
	}
	gotToolText := cur.UserInputMessageContext.ToolResults[0].Content[0].Text
	if gotToolText != "tool-result-structured" {
		t.Fatalf("expected structured tool result text, got %q", gotToolText)
	}
}

func TestOpenAIToKiroAssistantMapContentInHistory(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "u1"},
			{Role: "assistant", Content: map[string]interface{}{"type": "text", "text": "assistant-map"}},
			{Role: "user", Content: "u2"},
		},
	}

	payload := OpenAIToKiro(req, false)

	if len(payload.ConversationState.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(payload.ConversationState.History))
	}
	assistant := payload.ConversationState.History[1].AssistantResponseMessage
	if assistant == nil {
		t.Fatalf("expected second history entry to be assistant")
	}
	if assistant.Content != "assistant-map" {
		t.Fatalf("expected assistant map content preserved, got %q", assistant.Content)
	}
}

func TestOpenAIToKiroAssistantToolCallsDoNotInjectPlaceholder(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "find weather"},
			{
				Role:    "assistant",
				Content: nil,
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: "get_weather", Arguments: "{}"},
				}},
			},
			{Role: "user", Content: "continue"},
		},
	}

	payload := OpenAIToKiro(req, false)
	if len(payload.ConversationState.History) < 2 {
		t.Fatalf("expected history with assistant tool call")
	}
	assistant := payload.ConversationState.History[1].AssistantResponseMessage
	if assistant == nil {
		t.Fatalf("expected assistant history entry")
	}
	if assistant.Content != "" {
		t.Fatalf("expected empty assistant content for tool-call-only turn, got %q", assistant.Content)
	}
}

func TestOpenAIConversationIDStableFromAnchor(t *testing.T) {
	baseMessages := []OpenAIMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Build calculator"},
		{Role: "assistant", Content: "Sure"},
		{Role: "user", Content: "Continue"},
	}

	reqA := &OpenAIRequest{Model: "claude-sonnet-4.5", Messages: baseMessages}
	reqB := &OpenAIRequest{Model: "claude-sonnet-4.5", Messages: append(baseMessages, OpenAIMessage{Role: "assistant", Content: "Next step"})}

	payloadA := OpenAIToKiro(reqA, false)
	payloadB := OpenAIToKiro(reqB, false)

	if payloadA.ConversationState.ConversationID == "" || payloadB.ConversationState.ConversationID == "" {
		t.Fatalf("expected non-empty conversation IDs")
	}
	if payloadA.ConversationState.ConversationID != payloadB.ConversationState.ConversationID {
		t.Fatalf("expected stable conversation ID across turns, got %q vs %q", payloadA.ConversationState.ConversationID, payloadB.ConversationState.ConversationID)
	}
}

func TestClaudeConversationIDStableFromAnchor(t *testing.T) {
	reqA := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "sys",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
		},
	}
	reqB := &ClaudeRequest{
		Model:  "claude-sonnet-4.5",
		System: "sys",
		Messages: []ClaudeMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "ok"},
			{Role: "user", Content: "next"},
		},
	}

	payloadA := ClaudeToKiro(reqA, false)
	payloadB := ClaudeToKiro(reqB, false)

	if payloadA.ConversationState.ConversationID == "" || payloadB.ConversationState.ConversationID == "" {
		t.Fatalf("expected non-empty conversation IDs")
	}
	if payloadA.ConversationState.ConversationID != payloadB.ConversationState.ConversationID {
		t.Fatalf("expected stable conversation ID across turns, got %q vs %q", payloadA.ConversationState.ConversationID, payloadB.ConversationState.ConversationID)
	}
}

func TestOpenAIConversationIDRandomForSyntheticAnchor(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "assistant", Content: "prefill"},
		},
	}

	payloadA := OpenAIToKiro(req, false)
	payloadB := OpenAIToKiro(req, false)

	if payloadA.ConversationState.ConversationID == payloadB.ConversationState.ConversationID {
		t.Fatalf("expected synthetic anchor to generate non-deterministic conversation IDs")
	}
}

func TestClaudeToKiroDropsLeadingAssistantHistory(t *testing.T) {
	req := &ClaudeRequest{
		Model: "claude-sonnet-4.5",
		Messages: []ClaudeMessage{
			{Role: "assistant", Content: "prefill"},
			{Role: "user", Content: "real user message"},
		},
	}

	payload := ClaudeToKiro(req, false)

	if len(payload.ConversationState.History) != 0 {
		t.Fatalf("expected leading assistant-only history to be dropped, got %d entries", len(payload.ConversationState.History))
	}

	if strings.Contains(payload.ConversationState.CurrentMessage.UserInputMessage.Content, "Begin conversation") {
		t.Fatalf("unexpected synthetic Begin conversation injection in current content: %q", payload.ConversationState.CurrentMessage.UserInputMessage.Content)
	}
}

func TestToolResultsContinuationIncludesInstructionPrefix(t *testing.T) {
	req := &OpenAIRequest{
		Model: "claude-sonnet-4.5",
		Messages: []OpenAIMessage{
			{Role: "user", Content: "find data"},
			{Role: "assistant", ToolCalls: []ToolCall{{
				ID:   "call_1",
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Name: "fetch", Arguments: "{}"},
			}}},
			{Role: "tool", ToolCallID: "call_1", Content: "result-1"},
		},
	}

	payload := OpenAIToKiro(req, false)
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content

	if !strings.Contains(content, toolResultsContinuationPrefix) {
		t.Fatalf("expected tool continuation prefix, got %q", content)
	}
	if !strings.Contains(content, "result-1") {
		t.Fatalf("expected tool result text in continuation content, got %q", content)
	}
}

func TestOpenAIToKiroMergesDeveloperMessageAndMaxCompletionTokens(t *testing.T) {
	req := &OpenAIRequest{
		Model:               "gpt-4o",
		MaxTokens:           100,
		MaxCompletionTokens: 321,
		Messages: []OpenAIMessage{
			{Role: "developer", Content: "Follow the project rules."},
			{Role: "user", Content: "Hello"},
		},
	}

	payload := OpenAIToKiro(req, false)
	content := payload.ConversationState.CurrentMessage.UserInputMessage.Content
	if !strings.Contains(content, "Follow the project rules.") || !strings.Contains(content, "Hello") {
		t.Fatalf("expected developer instructions to be merged into current content, got %q", content)
	}
	if payload.InferenceConfig == nil {
		t.Fatalf("expected inference config")
	}
	if payload.InferenceConfig.MaxTokens != 321 {
		t.Fatalf("expected max_completion_tokens to take precedence, got %d", payload.InferenceConfig.MaxTokens)
	}
}

func TestCompletionsToOpenAIRequestConvertsPrompt(t *testing.T) {
	req := &OpenAICompletionsRequest{
		Model:     "gpt-4o",
		Prompt:    []interface{}{"first", "second"},
		MaxTokens: 42,
	}

	chatReq := CompletionsToOpenAIRequest(req)
	if chatReq.Model != "gpt-4o" || chatReq.MaxTokens != 42 {
		t.Fatalf("expected model and max tokens to be preserved, got %#v", chatReq)
	}
	if len(chatReq.Messages) != 1 {
		t.Fatalf("expected one user message, got %d", len(chatReq.Messages))
	}
	if chatReq.Messages[0].Role != "user" || chatReq.Messages[0].Content != "first\nsecond" {
		t.Fatalf("expected prompt array to become joined user content, got %#v", chatReq.Messages[0])
	}
}

func TestKiroToOpenAICompletionResponseShape(t *testing.T) {
	resp := KiroToOpenAICompletionResponse("hello", 4, 2, "gpt-4o")
	if resp["object"] != "text_completion" {
		t.Fatalf("expected text_completion object, got %#v", resp["object"])
	}
	choices, ok := resp["choices"].([]map[string]interface{})
	if !ok || len(choices) != 1 {
		t.Fatalf("expected one completion choice, got %#v", resp["choices"])
	}
	if choices[0]["text"] != "hello" || choices[0]["finish_reason"] != "stop" {
		t.Fatalf("unexpected completion choice: %#v", choices[0])
	}
}

func TestResponsesToOpenAIRequestConvertsInputAndFunctionTool(t *testing.T) {
	req := &OpenAIResponsesRequest{
		Model:           "gpt-4o",
		Input:           "Say hello",
		Instructions:    "Be concise.",
		MaxOutputTokens: 77,
		Tools: []OpenAIResponsesTool{{
			Type:        "function",
			Name:        "lookup",
			Description: "Look up data",
			Parameters: map[string]interface{}{
				"type": "object",
			},
		}},
	}

	chatReq := ResponsesToOpenAIRequest(req)
	if chatReq.Model != "gpt-4o" {
		t.Fatalf("expected model to be preserved, got %q", chatReq.Model)
	}
	if chatReq.MaxCompletionTokens != 77 {
		t.Fatalf("expected max_output_tokens to map to max_completion_tokens, got %d", chatReq.MaxCompletionTokens)
	}
	if len(chatReq.Messages) != 2 {
		t.Fatalf("expected developer+user messages, got %d", len(chatReq.Messages))
	}
	if chatReq.Messages[0].Role != "developer" || chatReq.Messages[0].Content != "Be concise." {
		t.Fatalf("expected instructions to become developer message, got %#v", chatReq.Messages[0])
	}
	if chatReq.Messages[1].Role != "user" || chatReq.Messages[1].Content != "Say hello" {
		t.Fatalf("expected string input to become user message, got %#v", chatReq.Messages[1])
	}
	if len(chatReq.Tools) != 1 || chatReq.Tools[0].Function.Name != "lookup" {
		t.Fatalf("expected function tool conversion, got %#v", chatReq.Tools)
	}
}

func TestResponsesToOpenAIRequestConvertsFunctionCallOutput(t *testing.T) {
	req := &OpenAIResponsesRequest{
		Model: "gpt-4o",
		Input: []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Find weather",
			},
			map[string]interface{}{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "get_weather",
				"arguments": map[string]interface{}{"city": "Paris"},
			},
			map[string]interface{}{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  "sunny",
			},
		},
	}

	chatReq := ResponsesToOpenAIRequest(req)
	if len(chatReq.Messages) != 3 {
		t.Fatalf("expected 3 converted messages, got %d", len(chatReq.Messages))
	}
	assistant := chatReq.Messages[1]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 {
		t.Fatalf("expected assistant tool call message, got %#v", assistant)
	}
	if assistant.ToolCalls[0].Function.Name != "get_weather" || !strings.Contains(assistant.ToolCalls[0].Function.Arguments, "Paris") {
		t.Fatalf("expected function call details to be preserved, got %#v", assistant.ToolCalls[0])
	}
	tool := chatReq.Messages[2]
	if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Content != "sunny" {
		t.Fatalf("expected function_call_output to become tool message, got %#v", tool)
	}
}

func TestKiroToOpenAIResponsesResponseShape(t *testing.T) {
	resp := KiroToOpenAIResponsesResponse("hello", "thinking", []KiroToolUse{{
		ToolUseID: "call_1",
		Name:      "lookup",
		Input:     map[string]interface{}{"q": "x"},
	}}, 10, 5, "gpt-4o")

	if resp["object"] != "response" || resp["status"] != "completed" {
		t.Fatalf("expected completed response object, got %#v", resp)
	}
	if resp["output_text"] != "hello" {
		t.Fatalf("expected output_text convenience field, got %#v", resp["output_text"])
	}
	output, ok := resp["output"].([]map[string]interface{})
	if !ok || len(output) != 2 {
		t.Fatalf("expected message and function_call output items, got %#v", resp["output"])
	}
	if output[0]["type"] != "message" || output[1]["type"] != "function_call" {
		t.Fatalf("expected response output items, got %#v", output)
	}
}

func TestKiroToOpenAIResponsesResponseCanReuseStreamID(t *testing.T) {
	resp := kiroToOpenAIResponsesResponseWithID("resp_fixed", "hello", "", nil, 1, 1, "gpt-4o")
	if resp["id"] != "resp_fixed" {
		t.Fatalf("expected response id to be reused, got %#v", resp["id"])
	}
}
