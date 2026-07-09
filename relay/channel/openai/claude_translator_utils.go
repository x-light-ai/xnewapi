package openai

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	jsonpkg "encoding/json"

	"github.com/QuantumNous/new-api/common"
	"github.com/tidwall/gjson"
)

var (
	claudeToolUseIDCounter uint64
)

func fixJSON(input string) string {
	var out bytes.Buffer

	inDouble := false
	inSingle := false
	escaped := false

	writeConverted := func(r rune) {
		if r == '"' {
			out.WriteByte('\\')
			out.WriteByte('"')
			return
		}
		out.WriteRune(r)
	}

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if inDouble {
			out.WriteRune(r)
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inDouble = false
			}
			continue
		}

		if inSingle {
			if escaped {
				escaped = false
				switch r {
				case 'n', 'r', 't', 'b', 'f', '/', '"':
					out.WriteByte('\\')
					out.WriteRune(r)
				case '\\':
					out.WriteByte('\\')
					out.WriteByte('\\')
				case '\'':
					out.WriteRune('\'')
				case 'u':
					out.WriteByte('\\')
					out.WriteByte('u')
					for k := 0; k < 4 && i+1 < len(runes); k++ {
						peek := runes[i+1]
						if (peek >= '0' && peek <= '9') || (peek >= 'a' && peek <= 'f') || (peek >= 'A' && peek <= 'F') {
							out.WriteRune(peek)
							i++
						} else {
							break
						}
					}
				default:
					out.WriteByte('\\')
					out.WriteRune(r)
				}
				continue
			}

			if r == '\\' {
				escaped = true
				continue
			}
			if r == '\'' {
				out.WriteByte('"')
				inSingle = false
				continue
			}
			writeConverted(r)
			continue
		}

		if r == '"' {
			inDouble = true
			out.WriteRune(r)
			continue
		}
		if r == '\'' {
			inSingle = true
			out.WriteByte('"')
			continue
		}
		out.WriteRune(r)
	}

	if inSingle {
		out.WriteByte('"')
	}

	return out.String()
}

func canonicalToolName(name string) string {
	canonical := strings.TrimSpace(name)
	canonical = strings.TrimLeft(canonical, "_")
	return strings.ToLower(canonical)
}

func toolNameMapFromClaudeRequest(rawJSON []byte) map[string]string {
	if len(rawJSON) == 0 || !gjson.ValidBytes(rawJSON) {
		return nil
	}

	tools := gjson.GetBytes(rawJSON, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}

	out := make(map[string]string, len(tools.Array()))
	tools.ForEach(func(_, tool gjson.Result) bool {
		name := strings.TrimSpace(tool.Get("name").String())
		if name == "" {
			name = strings.TrimSpace(tool.Get("function.name").String())
		}
		if name == "" {
			return true
		}
		key := canonicalToolName(name)
		if key == "" {
			return true
		}
		if _, exists := out[key]; !exists {
			out[key] = name
		}
		return true
	})

	if len(out) == 0 {
		return nil
	}
	return out
}

func mapToolName(toolNameMap map[string]string, name string) string {
	if name == "" || toolNameMap == nil {
		return name
	}
	if mapped, ok := toolNameMap[canonicalToolName(name)]; ok && mapped != "" {
		return mapped
	}
	return name
}

func sanitizeFunctionName(name string) string {
	if name == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range name {
		allowed := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == '.' || r == ':' || r == '-'
		if i > 0 {
			allowed = allowed || (r >= '0' && r <= '9')
		}
		if allowed {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	first := out[0]
	if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || first == '_') {
		out = "_" + out
		if len(out) > 64 {
			out = out[:64]
		}
	}
	return out
}

func sanitizedToolNameMap(rawJSON []byte) map[string]string {
	if len(rawJSON) == 0 || !gjson.ValidBytes(rawJSON) {
		return nil
	}

	tools := gjson.GetBytes(rawJSON, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}

	out := make(map[string]string)
	tools.ForEach(func(_, tool gjson.Result) bool {
		name := strings.TrimSpace(tool.Get("name").String())
		if name == "" {
			return true
		}
		sanitized := sanitizeFunctionName(name)
		if sanitized == "" || sanitized == name {
			return true
		}
		if _, exists := out[sanitized]; !exists {
			out[sanitized] = name
		}
		return true
	})

	if len(out) == 0 {
		return nil
	}
	return out
}

func restoreSanitizedToolName(toolNameMap map[string]string, sanitizedName string) string {
	if sanitizedName == "" || toolNameMap == nil {
		return sanitizedName
	}
	if original, ok := toolNameMap[sanitizedName]; ok {
		return original
	}
	return sanitizedName
}

func sanitizeClaudeToolID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		s = fmt.Sprintf("toolu_%d_%d", time.Now().UnixNano(), atomic.AddUint64(&claudeToolUseIDCounter, 1))
	}
	return s
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

func BoolPtr(v bool) *bool {
	return &v
}

func mapReasoningEffort(thinkingType string, budgetTokens *int, effort string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	switch thinkingType {
	case "adaptive", "auto":
		if effort != "" {
			return effort
		}
		return "high"
	case "disabled":
		return "none"
	case "enabled":
		if budgetTokens == nil {
			return "medium"
		}
		budget := *budgetTokens
		switch {
		case budget <= 0:
			return "none"
		case budget <= 2048:
			return "low"
		case budget <= 8192:
			return "medium"
		case budget <= 16384:
			return "high"
		default:
			return "high"
		}
	default:
		return ""
	}
}

func isClaudeCodeAttributionSystemText(text string) bool {
	text = strings.TrimLeftFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v'
	})
	return strings.HasPrefix(text, "x-anthropic-billing-header:")
}

func shouldMapClaudeThinkingToGPTReasoning(part gjson.Result) bool {
	signature := strings.TrimSpace(part.Get("signature").String())
	if signature == "" {
		return false
	}
	return isValidGPTReasoningSignature(signature)
}

func isValidGPTReasoningSignature(rawSignature string) bool {
	sig := strings.TrimSpace(rawSignature)
	if strings.HasPrefix(sig, "gpt#") || strings.HasPrefix(sig, "openai#") || strings.HasPrefix(sig, "codex#") {
		_, sig, _ = strings.Cut(sig, "#")
		sig = strings.TrimSpace(sig)
	}
	if sig == "" || len(sig) > 32*1024*1024 || !strings.HasPrefix(sig, "gAAAA") {
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

func decodeOutputConfigEffort(raw jsonpkg.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var cfg struct {
		Effort string `json:"effort,omitempty"`
	}
	if err := common.Unmarshal(raw, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Effort)
}

func toJSONString(v any) string {
	b, err := common.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

const (
	claudeSystemReminderStart = "<system-reminder>"
	claudeSystemReminderEnd   = "</system-reminder>"
)

// claudeMessageSystemReminderText converts a Claude message-level system value
// into ordinary user-visible reminder text for non-Claude upstream formats.
func claudeMessageSystemReminderText(content gjson.Result) (string, bool) {
	parts := claudeSystemTextParts(content)
	if len(parts) == 0 {
		return "", false
	}
	text := strings.Join(parts, "\n")
	if strings.TrimSpace(text) == "" {
		return "", false
	}
	return claudeSystemReminderStart + "\n" + text + "\n" + claudeSystemReminderEnd, true
}

func claudeSystemTextParts(content gjson.Result) []string {
	if !content.Exists() {
		return nil
	}
	if content.Type == gjson.String {
		text := content.String()
		if text == "" || isClaudeCodeAttributionSystemText(text) {
			return nil
		}
		return []string{text}
	}
	if !content.IsArray() {
		return nil
	}
	parts := make([]string, 0)
	content.ForEach(func(_, item gjson.Result) bool {
		if item.Get("type").String() != "text" {
			return true
		}
		text := item.Get("text").String()
		if text == "" || isClaudeCodeAttributionSystemText(text) {
			return true
		}
		parts = append(parts, text)
		return true
	})
	return parts
}
