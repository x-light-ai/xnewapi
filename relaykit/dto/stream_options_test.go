// FORK-CUSTOM: Verify explicit false values survive StreamOptions marshaling.
package dto_test

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	kitutil "github.com/QuantumNous/new-api/relaykit/relayconvert/kitutil"
	"github.com/stretchr/testify/require"
)

func TestStreamOptions_ExplicitFalsePreserved(t *testing.T) {
	cases := []struct {
		name     string
		opts     *dto.StreamOptions
		wantJSON string
	}{
		{"nil pointer omitted", nil, `{}`},
		{"nil IncludeUsage omitted", &dto.StreamOptions{}, `{}`},
		{"explicit false preserved", &dto.StreamOptions{IncludeUsage: kitutil.GetPointer(false), IncludeObfuscation: kitutil.GetPointer(false)}, `{"include_usage":false,"include_obfuscation":false}`},
		{"explicit true preserved", &dto.StreamOptions{IncludeUsage: kitutil.GetPointer(true)}, `{"include_usage":true}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapper := struct {
				SO *dto.StreamOptions `json:"stream_options,omitempty"`
			}{SO: tc.opts}
			b, err := kitutil.Marshal(wrapper)
			require.NoError(t, err)
			if tc.opts == nil {
				require.JSONEq(t, `{}`, string(b))
			} else {
				var got map[string]any
				require.NoError(t, kitutil.Unmarshal(b, &got))
				soRaw, _ := kitutil.Marshal(got["stream_options"])
				require.JSONEq(t, tc.wantJSON, string(soRaw))
			}
		})
	}
}
