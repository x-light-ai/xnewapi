package codex

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

const claudeCodeAttributionSystemPrefix = "x-anthropic-billing-header:"

var (
	claudeToolUseIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	claudeToolUseIDCounter   uint64
)

func isClaudeCodeAttributionSystemText(text string) bool {
	text = strings.TrimLeftFunc(text, unicode.IsSpace)
	return strings.HasPrefix(text, claudeCodeAttributionSystemPrefix)
}

func sanitizeClaudeToolID(id string) string {
	s := claudeToolUseIDSanitizer.ReplaceAllString(id, "_")
	if s == "" {
		s = fmt.Sprintf("toolu_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&claudeToolUseIDCounter, 1))
	}
	return s
}

func convertBudgetToLevel(budget int) (string, bool) {
	switch {
	case budget < -1:
		return "", false
	case budget == -1:
		return "auto", true
	case budget == 0:
		return "none", true
	case budget <= 512:
		return "minimal", true
	case budget <= 1024:
		return "low", true
	case budget <= 8192:
		return "medium", true
	case budget <= 24576:
		return "high", true
	default:
		return "xhigh", true
	}
}

func compatibleGPTSignature(rawSignature string) (string, bool) {
	payload := signaturePayloadWithoutProviderPrefix(rawSignature)
	if !isValidGPTReasoningSignature(payload) {
		return "", false
	}
	return payload, true
}

func signaturePayloadWithoutProviderPrefix(rawSignature string) string {
	prefix, rest, ok := strings.Cut(strings.TrimSpace(rawSignature), "#")
	if !ok {
		return strings.TrimSpace(rawSignature)
	}
	switch strings.ToLower(strings.TrimSpace(prefix)) {
	case "openai", "gpt", "codex":
		return strings.TrimSpace(rest)
	default:
		return strings.TrimSpace(rawSignature)
	}
}

func isValidGPTReasoningSignature(rawSignature string) bool {
	sig := strings.TrimSpace(rawSignature)
	if sig == "" || len(sig) > 32*1024*1024 {
		return false
	}
	for _, r := range sig {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '=':
		default:
			return false
		}
	}
	if !strings.HasPrefix(sig, "gAAAA") {
		return false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(sig)
	}
	if err != nil || len(decoded) < 73 || decoded[0] != 0x80 {
		return false
	}
	ciphertextLen := len(decoded) - 1 - 8 - 16 - 32
	return ciphertextLen > 0 && ciphertextLen%16 == 0
}

func appendSSEEventBytes(out []byte, event string, payload []byte, trailingNewlines int) []byte {
	out = append(out, "event: "...)
	out = append(out, event...)
	out = append(out, '\n')
	out = append(out, "data: "...)
	out = append(out, payload...)
	for i := 0; i < trailingNewlines; i++ {
		out = append(out, '\n')
	}
	return out
}

func claudeInputTokensJSON(count int64) []byte {
	out := make([]byte, 0, 32)
	out = append(out, `{"input_tokens":`...)
	out = strconv.AppendInt(out, count, 10)
	out = append(out, '}')
	return out
}
