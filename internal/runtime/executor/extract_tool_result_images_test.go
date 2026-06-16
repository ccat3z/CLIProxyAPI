package executor

import (
	"encoding/json"
	"testing"

	"github.com/tidwall/gjson"
)

func TestExtractToolResultImages_ImageOnly(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"messages": [
			{"role": "user", "content": "read the file"},
			{"role": "assistant", "content": [{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": {}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "abc123"}}]}]},
			{"role": "assistant", "content": [{"type": "text", "text": "I can see the image."}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	msgs := gjson.GetBytes(result, "messages").Array()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	// Message 2 should still be tool_result but with placeholder text
	toolResult := msgs[2].Get("content.0")
	if toolResult.Get("type").String() != "tool_result" {
		t.Fatalf("expected tool_result type, got %s", toolResult.Get("type").String())
	}
	innerContent := toolResult.Get("content").Array()
	if len(innerContent) != 1 {
		t.Fatalf("expected 1 inner content item, got %d", len(innerContent))
	}
	if innerContent[0].Get("type").String() != "text" {
		t.Fatalf("expected text placeholder, got %s", innerContent[0].Get("type").String())
	}
	if innerContent[0].Get("text").String() != "(image result attached below)" {
		t.Fatalf("expected placeholder text, got %s", innerContent[0].Get("text").String())
	}

	// Message 3 should be the extracted image as a user message
	inserted := msgs[3]
	if inserted.Get("role").String() != "user" {
		t.Fatalf("expected user role, got %s", inserted.Get("role").String())
	}
	imgContent := inserted.Get("content").Array()
	if len(imgContent) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgContent))
	}
	if imgContent[0].Get("type").String() != "image" {
		t.Fatalf("expected image type, got %s", imgContent[0].Get("type").String())
	}
}

func TestExtractToolResultImages_TextAndImage(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"messages": [
			{"role": "user", "content": "read the file"},
			{"role": "assistant", "content": [{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": {}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "text", "text": "file content here"}, {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "abc123"}}]}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Done."}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	msgs := gjson.GetBytes(result, "messages").Array()
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}

	// tool_result should keep text only
	toolResult := msgs[2].Get("content.0")
	innerContent := toolResult.Get("content").Array()
	if len(innerContent) != 1 {
		t.Fatalf("expected 1 inner content item (text), got %d", len(innerContent))
	}
	if innerContent[0].Get("type").String() != "text" {
		t.Fatalf("expected text type, got %s", innerContent[0].Get("type").String())
	}
	if innerContent[0].Get("text").String() != "file content here" {
		t.Fatalf("expected original text, got %s", innerContent[0].Get("text").String())
	}

	// Inserted user message should have the image
	inserted := msgs[3]
	imgContent := inserted.Get("content").Array()
	if len(imgContent) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgContent))
	}
	if imgContent[0].Get("type").String() != "image" {
		t.Fatalf("expected image type, got %s", imgContent[0].Get("type").String())
	}
}

func TestExtractToolResultImages_NoImage(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"messages": [
			{"role": "user", "content": "read the file"},
			{"role": "assistant", "content": [{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": {}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "text", "text": "file content"}]}]},
			{"role": "assistant", "content": [{"type": "text", "text": "Done."}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	msgs := gjson.GetBytes(result, "messages").Array()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages (unchanged), got %d", len(msgs))
	}
}

func TestExtractToolResultImages_NoToolResult(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"messages": [
			{"role": "user", "content": "hello"},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi there."}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	msgs := gjson.GetBytes(result, "messages").Array()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (unchanged), got %d", len(msgs))
	}
}

func TestExtractToolResultImages_MultipleImages(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"messages": [
			{"role": "user", "content": "read the file"},
			{"role": "assistant", "content": [{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": {}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "img1"}}, {"type": "image", "source": {"type": "base64", "media_type": "image/jpeg", "data": "img2"}}]}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	msgs := gjson.GetBytes(result, "messages").Array()
	if len(msgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(msgs))
	}

	// Inserted user message should have both images
	inserted := msgs[3]
	imgContent := inserted.Get("content").Array()
	if len(imgContent) != 2 {
		t.Fatalf("expected 2 images, got %d", len(imgContent))
	}
}

func TestExtractToolResultImages_PreservesOtherFields(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"max_tokens": 32000,
		"system": [{"type": "text", "text": "You are helpful."}],
		"messages": [
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "abc"}}]}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	if gjson.GetBytes(result, "model").String() != "kimi-k2.6" {
		t.Fatal("model field should be preserved")
	}
	if gjson.GetBytes(result, "max_tokens").Int() != 32000 {
		t.Fatal("max_tokens field should be preserved")
	}
	if !gjson.GetBytes(result, "system").Exists() {
		t.Fatal("system field should be preserved")
	}
}

func TestExtractToolResultImages_EmptyInput(t *testing.T) {
	result := extractToolResultImages(nil)
	if result != nil {
		t.Fatalf("expected nil for nil input, got %v", result)
	}

	result = extractToolResultImages([]byte{})
	if len(result) != 0 {
		t.Fatalf("expected empty for empty input, got %v", result)
	}

	result = extractToolResultImages([]byte("not json"))
	if string(result) != "not json" {
		t.Fatalf("expected invalid json to pass through, got %s", string(result))
	}
}

func TestExtractToolResultImages_StructureIsValid(t *testing.T) {
	input := `{
		"model": "kimi-k2.6",
		"messages": [
			{"role": "user", "content": "read the file"},
			{"role": "assistant", "content": [{"type": "tool_use", "id": "toolu_1", "name": "Read", "input": {}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "toolu_1", "content": [{"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "abc123"}}]}]},
			{"role": "assistant", "content": [{"type": "text", "text": "I can see the image."}]}
		]
	}`

	result := extractToolResultImages([]byte(input))

	if !json.Valid(result) {
		t.Fatalf("result is not valid JSON: %s", string(result))
	}
}
