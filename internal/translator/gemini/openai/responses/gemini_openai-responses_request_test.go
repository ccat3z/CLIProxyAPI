package responses

import (
	"encoding/base64"
	"testing"

	"github.com/tidwall/gjson"
)

const testResponsesGeminiThoughtSignature = "EjQKMgEMOdbHO0Gd+c9Mxk4ELwPGbpCEcp2mFfYYLix2UVtBH3fL8GECc4+JITVnHF4qZDsA"

func TestConvertOpenAIResponsesRequestToGemini_ReasoningSignatureCompatibility(t *testing.T) {
	tests := []struct {
		name          string
		encrypted     string
		wantSignature string
	}{
		{
			name:          "GPT encrypted_content uses Gemini bypass",
			encrypted:     validResponsesGPTReasoningSignature(),
			wantSignature: geminiResponsesThoughtSignature,
		},
		{
			name:          "Gemini encrypted_content is preserved",
			encrypted:     "gemini#" + testResponsesGeminiThoughtSignature,
			wantSignature: testResponsesGeminiThoughtSignature,
		},
		{
			name:          "Missing encrypted_content uses Gemini bypass",
			encrypted:     "",
			wantSignature: geminiResponsesThoughtSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(`{
				"model": "gpt-5",
				"input": [{
					"type": "reasoning",
					"encrypted_content": "` + tt.encrypted + `",
					"summary": [{"type": "summary_text", "text": "reasoning summary"}]
				}]
			}`)

			output := ConvertOpenAIResponsesRequestToGemini("gemini-3.5-flash", input, false)
			part := gjson.GetBytes(output, "contents.0.parts.0")
			if got := part.Get("thoughtSignature").String(); got != tt.wantSignature {
				t.Fatalf("thoughtSignature = %q, want %q. Output: %s", got, tt.wantSignature, output)
			}
			if got := part.Get("text").String(); got != "reasoning summary" {
				t.Fatalf("thought text = %q, want reasoning summary. Output: %s", got, output)
			}
		})
	}
}

func validResponsesGPTReasoningSignature() string {
	raw := make([]byte, 1+8+16+16+32)
	raw[0] = 0x80
	raw[8] = 1
	for i := 9; i < len(raw); i++ {
		raw[i] = byte(i)
	}
	return base64.URLEncoding.EncodeToString(raw)
}

func TestConvertOpenAIResponsesRequestToGemini_SystemAndDeveloperRoles(t *testing.T) {
	// Test system role conversion
	systemInput := []byte(`{
		"instructions": "Be a helpful assistant",
		"input": [
			{
				"type": "message",
				"role": "system",
				"content": [
					{
						"type": "input_text",
						"text": "System message text"
					}
				]
			},
			{
				"type": "message",
				"role": "user",
				"content": [
					{
						"type": "input_text",
						"text": "Hello"
					}
				]
			}
		]
	}`)

	outSystem := ConvertOpenAIResponsesRequestToGemini("gemini-3.5-flash", systemInput, false)
	resSystem := gjson.ParseBytes(outSystem)

	systemInstruction := resSystem.Get("systemInstruction")
	if !systemInstruction.Exists() {
		t.Errorf("Expected systemInstruction field to exist")
	}
	parts := systemInstruction.Get("parts")
	if parts.Get("#").Int() != 2 {
		t.Errorf("Expected 2 parts in systemInstruction, got %d", parts.Get("#").Int())
	}
	if parts.Get("0.text").String() != "Be a helpful assistant" {
		t.Errorf("Expected first part to be 'Be a helpful assistant', got '%s'", parts.Get("0.text").String())
	}
	if parts.Get("1.text").String() != "System message text" {
		t.Errorf("Expected second part to be 'System message text', got '%s'", parts.Get("1.text").String())
	}

	// Test developer role conversion (which is the main bug we're addressing)
	developerInput := []byte(`{
		"instructions": "Be a helpful assistant",
		"input": [
			{
				"type": "message",
				"role": "developer",
				"content": [
					{
						"type": "input_text",
						"text": "Developer message text"
					}
				]
			},
			{
				"type": "message",
				"role": "user",
				"content": [
					{
						"type": "input_text",
						"text": "Hello"
					}
				]
			}
		]
	}`)

	outDev := ConvertOpenAIResponsesRequestToGemini("gemini-3.5-flash", developerInput, false)
	resDev := gjson.ParseBytes(outDev)

	systemInstructionDev := resDev.Get("systemInstruction")
	if !systemInstructionDev.Exists() {
		t.Errorf("Expected systemInstruction field to exist for developer role")
	}
	partsDev := systemInstructionDev.Get("parts")
	if partsDev.Get("#").Int() != 2 {
		t.Errorf("Expected 2 parts in systemInstruction for developer role, got %d", partsDev.Get("#").Int())
	}
	if partsDev.Get("0.text").String() != "Be a helpful assistant" {
		t.Errorf("Expected first part to be 'Be a helpful assistant', got '%s'", partsDev.Get("0.text").String())
	}
	if partsDev.Get("1.text").String() != "Developer message text" {
		t.Errorf("Expected second part to be 'Developer message text', got '%s'", partsDev.Get("1.text").String())
	}

	// Ensure role 'developer' is not sent inside contents array as a regular message
	contents := resDev.Get("contents")
	contents.ForEach(func(_, value gjson.Result) bool {
		role := value.Get("role").String()
		if role == "developer" {
			t.Errorf("Role 'developer' leaked into contents array")
		}
		return true
	})
}
