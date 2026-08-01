// FORK-CUSTOM: Verify CPA-derived parallel Codex function-call serialization.
package codex

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type observedClaudeContentBlock struct {
	Index     int64
	Type      string
	ID        string
	Name      string
	Text      string
	Arguments string
}

func translateCodexChunksForTest(t *testing.T, chunks ...string) [][]byte {
	t.Helper()

	originalRequest := []byte(`{"stream":true,"tools":[{"name":"Read"}]}`)
	var state any
	var outputs [][]byte
	for _, chunk := range chunks {
		outputs = append(outputs, ConvertCodexResponseToClaude(
			context.Background(), "gpt-5", originalRequest, nil, []byte(chunk), &state,
		)...)
	}
	return outputs
}

func observeClaudeContentBlocks(t *testing.T, outputs [][]byte) []*observedClaudeContentBlock {
	t.Helper()

	open := make(map[int64]*observedClaudeContentBlock)
	started := make(map[int64]struct{})
	blocks := make([]*observedClaudeContentBlock, 0)
	messageState := 0
	for _, output := range outputs {
		for _, line := range strings.Split(string(output), "\n") {
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			event := gjson.Parse(strings.TrimPrefix(line, "data: "))
			require.NotEqual(t, 2, messageState, "event emitted after message_stop: %s", event.Raw)
			index := event.Get("index").Int()
			switch event.Get("type").String() {
			case "content_block_start":
				require.Zero(t, messageState, "content block started after terminal event: %s", event.Raw)
				require.Empty(t, open, "content block started while another remains open")
				_, reused := started[index]
				require.False(t, reused, "content block index %d was reused", index)
				block := &observedClaudeContentBlock{
					Index: index,
					Type:  event.Get("content_block.type").String(),
					ID:    event.Get("content_block.id").String(),
					Name:  event.Get("content_block.name").String(),
				}
				open[index] = block
				started[index] = struct{}{}
				blocks = append(blocks, block)
			case "content_block_delta":
				block := open[index]
				require.NotNil(t, block, "delta targets unopened block %d", index)
				switch event.Get("delta.type").String() {
				case "input_json_delta":
					block.Arguments += event.Get("delta.partial_json").String()
				case "text_delta":
					block.Text += event.Get("delta.text").String()
				}
			case "content_block_stop":
				require.Contains(t, open, index, "stop targets unopened block %d", index)
				delete(open, index)
			case "message_delta":
				require.Empty(t, open, "message_delta emitted with open content blocks")
				require.Zero(t, messageState, "duplicate message_delta")
				messageState = 1
			case "message_stop":
				require.Empty(t, open, "message_stop emitted with open content blocks")
				require.Equal(t, 1, messageState, "message_stop emitted before message_delta")
				messageState = 2
			}
		}
	}
	require.Empty(t, open, "content blocks remain open")
	return blocks
}

func TestConvertCodexResponseToClaude_SerializesInterleavedFunctionCalls(t *testing.T) {
	tests := map[string][]string{
		"first call finishes first": {
			`data: {"type":"response.created","response":{"id":"resp_parallel","model":"gpt-5"}}`,
			`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_a","name":"Read"},"output_index":1}`,
			`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_b","name":"Read"},"output_index":2}`,
			`data: {"type":"response.function_call_arguments.delta","delta":"{\"file_path\":\"a\"}","output_index":1}`,
			`data: {"type":"response.function_call_arguments.delta","delta":"{\"file_path\":\"b\"}","output_index":2}`,
			`data: {"type":"response.function_call_arguments.done","arguments":"{\"file_path\":\"a\"}","output_index":1}`,
			`data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_a","name":"Read","arguments":"{\"file_path\":\"a\"}"},"output_index":1}`,
			`data: {"type":"response.function_call_arguments.done","arguments":"{\"file_path\":\"b\"}","output_index":2}`,
			`data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_b","name":"Read","arguments":"{\"file_path\":\"b\"}"},"output_index":2}`,
		},
		"second call finishes first": {
			`data: {"type":"response.created","response":{"id":"resp_parallel","model":"gpt-5"}}`,
			`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_a","name":"Read"},"output_index":1}`,
			`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_b","name":"Read"},"output_index":2}`,
			`data: {"type":"response.function_call_arguments.delta","delta":"{\"file_path\":\"b\"}","output_index":2}`,
			`data: {"type":"response.function_call_arguments.done","arguments":"{\"file_path\":\"b\"}","output_index":2}`,
			`data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_b","name":"Read","arguments":"{\"file_path\":\"b\"}"},"output_index":2}`,
			`data: {"type":"response.function_call_arguments.delta","delta":"{\"file_path\":\"a\"}","output_index":1}`,
			`data: {"type":"response.function_call_arguments.done","arguments":"{\"file_path\":\"a\"}","output_index":1}`,
			`data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_a","name":"Read","arguments":"{\"file_path\":\"a\"}"},"output_index":1}`,
		},
	}

	for name, chunks := range tests {
		t.Run(name, func(t *testing.T) {
			blocks := observeClaudeContentBlocks(t, translateCodexChunksForTest(t, chunks...))
			require.Equal(t, []*observedClaudeContentBlock{
				{Index: 0, Type: "tool_use", ID: "call_a", Name: "Read", Arguments: `{"file_path":"a"}`},
				{Index: 1, Type: "tool_use", ID: "call_b", Name: "Read", Arguments: `{"file_path":"b"}`},
			}, blocks)
		})
	}
}

func TestConvertCodexResponseToClaude_DefersTextUntilFunctionCallCloses(t *testing.T) {
	outputs := translateCodexChunksForTest(t,
		`data: {"type":"response.created","response":{"id":"resp_mixed","model":"gpt-5"}}`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_a","name":"Read"},"output_index":0}`,
		`data: {"type":"response.content_part.added","part":{"type":"output_text"},"output_index":1}`,
		`data: {"type":"response.output_text.delta","delta":"done","output_index":1}`,
		`data: {"type":"response.content_part.done","part":{"type":"output_text"},"output_index":1}`,
		`data: {"type":"response.function_call_arguments.done","arguments":"{\"file_path\":\"a\"}","output_index":0}`,
		`data: {"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_a","name":"Read","arguments":"{\"file_path\":\"a\"}"},"output_index":0}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1}}}`,
	)

	blocks := observeClaudeContentBlocks(t, outputs)
	require.Equal(t, []*observedClaudeContentBlock{
		{Index: 0, Type: "tool_use", ID: "call_a", Name: "Read", Arguments: `{"file_path":"a"}`},
		{Index: 1, Type: "text", Text: "done"},
	}, blocks)
}

func TestConvertCodexResponseToClaude_TerminalHydratesParallelFunctionCalls(t *testing.T) {
	outputs := translateCodexChunksForTest(t,
		`data: {"type":"response.created","response":{"id":"resp_parallel","model":"gpt-5"}}`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_a","name":"Read"},"output_index":0}`,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","call_id":"call_b","name":"Read"},"output_index":1}`,
		`data: {"type":"response.function_call_arguments.delta","delta":"{\"file_path\":","output_index":0}`,
		`data: {"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1},"output":[{"type":"function_call","call_id":"call_a","name":"Read","arguments":"{\"file_path\":\"a\"}"},{"type":"function_call","call_id":"call_b","name":"Read","arguments":"{\"file_path\":\"b\"}"}]}}`,
	)

	blocks := observeClaudeContentBlocks(t, outputs)
	require.Equal(t, []*observedClaudeContentBlock{
		{Index: 0, Type: "tool_use", ID: "call_a", Name: "Read", Arguments: `{"file_path":"a"}`},
		{Index: 1, Type: "tool_use", ID: "call_b", Name: "Read", Arguments: `{"file_path":"b"}`},
	}, blocks)
}
