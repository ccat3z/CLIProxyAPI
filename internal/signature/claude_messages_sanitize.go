package signature

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type SignatureSanitizeReport struct {
	TargetProvider     SignatureProvider
	Preserved          int
	DroppedBlocks      int
	DroppedSignatures  int
	ReplacedSignatures int
	Decisions          []SignatureCompatibilityDecision
}

// SanitizeClaudeMessagesForClaudeUpstream prepares a Claude /v1/messages body
// for native Claude upstreams. Invalid thinking blocks are dropped, valid
// thinking signatures are normalized to Claude provider-native E-form, and
// tool_use blocks keep only their tool-call payload.
func SanitizeClaudeMessagesForClaudeUpstream(payload []byte, targetModel string) ([]byte, SignatureSanitizeReport) {
	const targetProvider = SignatureProviderClaude
	report := SignatureSanitizeReport{TargetProvider: targetProvider}

	messages := gjson.GetBytes(payload, "messages")
	if !messages.IsArray() {
		return payload, report
	}

	messageResults := messages.Array()
	keptMessages := make([]string, 0, len(messageResults))
	modified := false

	for i, message := range messageResults {
		content := message.Get("content")
		if !content.IsArray() {
			keptMessages = append(keptMessages, message.Raw)
			continue
		}

		contentResults := content.Array()
		keptParts := make([]string, 0, len(contentResults))
		messageModified := false

		for j, part := range contentResults {
			partType := part.Get("type").String()
			if partType == "tool_use" {
				updatedPart, changed := stripClaudeToolUseSignatureFields(part)
				if changed {
					messageModified = true
					report.DroppedSignatures++
				}
				keptParts = append(keptParts, updatedPart)
				continue
			}

			if partType != "thinking" {
				keptParts = append(keptParts, part.Raw)
				continue
			}

			rawSignature := part.Get("signature").String()
			decision := DecideSignatureCompatibility(targetProvider, rawSignature, SignatureBlockKindClaudeThinking)
			decision.Reason = fmt.Sprintf("messages[%d].content[%d]: %s", i, j, decision.Reason)
			report.Decisions = append(report.Decisions, decision)

			switch decision.Action {
			case SignatureActionPreserve:
				report.Preserved++
				if decision.NormalizedSignature != "" && decision.NormalizedSignature != rawSignature {
					updated, _ := sjson.Set(part.Raw, "signature", decision.NormalizedSignature)
					keptParts = append(keptParts, updated)
					messageModified = true
					continue
				}
				keptParts = append(keptParts, part.Raw)
			case SignatureActionReplaceWithGeminiBypass:
				report.ReplacedSignatures++
				updated, _ := sjson.Set(part.Raw, "signature", decision.ReplacementSignature)
				keptParts = append(keptParts, updated)
				messageModified = true
			case SignatureActionDropSignature:
				report.DroppedSignatures++
				updated, _ := sjson.Delete(part.Raw, "signature")
				keptParts = append(keptParts, updated)
				messageModified = true
			default:
				report.DroppedBlocks++
				messageModified = true
			}
		}

		if messageModified {
			modified = true
			if len(keptParts) == 0 {
				continue
			}
			updated, _ := sjson.SetRaw(message.Raw, "content", "["+strings.Join(keptParts, ",")+"]")
			keptMessages = append(keptMessages, updated)
			continue
		}

		keptMessages = append(keptMessages, message.Raw)
	}

	if !modified {
		return payload, report
	}
	output, _ := sjson.SetRawBytes(payload, "messages", []byte("["+strings.Join(keptMessages, ",")+"]"))
	return output, report
}

func stripClaudeToolUseSignatureFields(part gjson.Result) (string, bool) {
	updated := part.Raw
	changed := false
	for _, sigPath := range claudeToolUseProvenancePaths() {
		if !gjson.Get(updated, sigPath).Exists() {
			continue
		}
		updated, _ = sjson.Delete(updated, sigPath)
		changed = true
	}
	if cleaned, ok := deleteEmptyJSONObjectPath(updated, "extra_content.google"); ok {
		updated = cleaned
		changed = true
	}
	if cleaned, ok := deleteEmptyJSONObjectPath(updated, "extra_content"); ok {
		updated = cleaned
		changed = true
	}
	return updated, changed
}

func claudeToolUseSignaturePaths() []string {
	return []string{
		"signature",
		"thoughtSignature",
		"thought_signature",
		"extra_content.google.thought_signature",
	}
}

func claudeToolUseProvenancePaths() []string {
	return append(claudeToolUseSignaturePaths(), "model")
}

func deleteEmptyJSONObjectPath(raw, path string) (string, bool) {
	result := gjson.Get(raw, path)
	if !result.Exists() || !result.IsObject() || len(result.Map()) != 0 {
		return raw, false
	}
	updated, err := sjson.Delete(raw, path)
	if err != nil {
		return raw, false
	}
	return updated, true
}
