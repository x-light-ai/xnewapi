package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestStreamOptions_ExplicitFalsePreserved(t *testing.T) {
	cases := []struct {
		name     string
		opts     *StreamOptions
		wantJSON string
	}{
		{"nil pointer omitted", nil, `{}`},
		{"nil IncludeUsage omitted", &StreamOptions{}, `{}`},
		{"explicit false preserved", &StreamOptions{IncludeUsage: BoolPtr(false)}, `{"include_usage":false}`},
		{"explicit true preserved", &StreamOptions{IncludeUsage: BoolPtr(true)}, `{"include_usage":true}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapper := struct {
				SO *StreamOptions `json:"stream_options,omitempty"`
			}{SO: tc.opts}
			b, err := common.Marshal(wrapper)
			require.NoError(t, err)
			if tc.opts == nil {
				require.JSONEq(t, `{}`, string(b))
			} else {
				var got map[string]any
				require.NoError(t, common.Unmarshal(b, &got))
				soRaw, _ := common.Marshal(got["stream_options"])
				require.JSONEq(t, tc.wantJSON, string(soRaw))
			}
		})
	}
}

func BoolPtr(v bool) *bool { return &v }
