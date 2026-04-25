package helps

import (
	"context"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/usage"
)

func TestParseOpenAIUsageChatCompletions(t *testing.T) {
	data := []byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":5}}}`)
	detail := ParseOpenAIUsage(data)
	if detail.InputTokens != 1 {
		t.Fatalf("input tokens = %d, want %d", detail.InputTokens, 1)
	}
	if detail.OutputTokens != 2 {
		t.Fatalf("output tokens = %d, want %d", detail.OutputTokens, 2)
	}
	if detail.TotalTokens != 3 {
		t.Fatalf("total tokens = %d, want %d", detail.TotalTokens, 3)
	}
	if detail.CachedTokens != 4 {
		t.Fatalf("cached tokens = %d, want %d", detail.CachedTokens, 4)
	}
	if detail.ReasoningTokens != 5 {
		t.Fatalf("reasoning tokens = %d, want %d", detail.ReasoningTokens, 5)
	}
}

func TestParseOpenAIUsageResponses(t *testing.T) {
	data := []byte(`{"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30,"input_tokens_details":{"cached_tokens":7},"output_tokens_details":{"reasoning_tokens":9}}}`)
	detail := ParseOpenAIUsage(data)
	if detail.InputTokens != 10 {
		t.Fatalf("input tokens = %d, want %d", detail.InputTokens, 10)
	}
	if detail.OutputTokens != 20 {
		t.Fatalf("output tokens = %d, want %d", detail.OutputTokens, 20)
	}
	if detail.TotalTokens != 30 {
		t.Fatalf("total tokens = %d, want %d", detail.TotalTokens, 30)
	}
	if detail.CachedTokens != 7 {
		t.Fatalf("cached tokens = %d, want %d", detail.CachedTokens, 7)
	}
	if detail.ReasoningTokens != 9 {
		t.Fatalf("reasoning tokens = %d, want %d", detail.ReasoningTokens, 9)
	}
}

func TestParseOpenAIStreamUsage_NullUsage(t *testing.T) {
	line := []byte(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","usage":null}`)
	detail, ok := ParseOpenAIStreamUsage(line)
	if ok {
		t.Fatalf("expected ok=false for null usage, got ok=true, detail=%+v", detail)
	}
}

func TestParseOpenAIStreamUsage_ValidUsage(t *testing.T) {
	line := []byte(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","usage":{"prompt_tokens":5,"completion_tokens":10,"total_tokens":15}}`)
	detail, ok := ParseOpenAIStreamUsage(line)
	if !ok {
		t.Fatal("expected ok=true for valid usage")
	}
	if detail.InputTokens != 5 {
		t.Fatalf("input tokens = %d, want 5", detail.InputTokens)
	}
	if detail.OutputTokens != 10 {
		t.Fatalf("output tokens = %d, want 10", detail.OutputTokens)
	}
	if detail.TotalTokens != 15 {
		t.Fatalf("total tokens = %d, want 15", detail.TotalTokens)
	}
}

func TestParseOpenAIStreamUsage_NoUsage(t *testing.T) {
	line := []byte(`data: {"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[]}`)
	_, ok := ParseOpenAIStreamUsage(line)
	if ok {
		t.Fatal("expected ok=false when usage key is absent")
	}
}

func TestParseClaudeStreamUsage_NullUsage(t *testing.T) {
	line := []byte(`event: message_start\ndata: {"type":"message_start","message":{"usage":null}}`)
	_, ok := ParseClaudeStreamUsage(line)
	if ok {
		t.Fatal("expected ok=false for null usage")
	}
}

func TestParseClaudeStreamUsage_ValidUsage(t *testing.T) {
	line := []byte(`data: {"type":"message_delta","usage":{"input_tokens":3,"output_tokens":7}}`)
	detail, ok := ParseClaudeStreamUsage(line)
	if !ok {
		t.Fatal("expected ok=true for valid usage")
	}
	if detail.InputTokens != 3 {
		t.Fatalf("input tokens = %d, want 3", detail.InputTokens)
	}
	if detail.OutputTokens != 7 {
		t.Fatalf("output tokens = %d, want 7", detail.OutputTokens)
	}
}

func TestParseGeminiStreamUsage_NullUsageMetadata(t *testing.T) {
	line := []byte(`data: {"candidates":[],"usageMetadata":null}`)
	_, ok := ParseGeminiStreamUsage(line)
	if ok {
		t.Fatal("expected ok=false for null usageMetadata")
	}
}

func TestParseGeminiCLIStreamUsage_NullUsageMetadata(t *testing.T) {
	line := []byte(`data: {"response":{"usageMetadata":null}}`)
	_, ok := ParseGeminiCLIStreamUsage(line)
	if ok {
		t.Fatal("expected ok=false for null usageMetadata")
	}
}

func TestParseAntigravityStreamUsage_NullUsageMetadata(t *testing.T) {
	line := []byte(`data: {"response":{"usageMetadata":null}}`)
	_, ok := ParseAntigravityStreamUsage(line)
	if ok {
		t.Fatal("expected ok=false for null usageMetadata")
	}
}

func TestUsageReporterBuildRecordIncludesLatency(t *testing.T) {
	reporter := &UsageReporter{
		provider:    "openai",
		model:       "gpt-5.4",
		requestedAt: time.Now().Add(-1500 * time.Millisecond),
	}

	record := reporter.buildRecord(context.Background(), usage.Detail{TotalTokens: 3}, false)
	if record.Latency < time.Second {
		t.Fatalf("latency = %v, want >= 1s", record.Latency)
	}
	if record.Latency > 3*time.Second {
		t.Fatalf("latency = %v, want <= 3s", record.Latency)
	}
}
