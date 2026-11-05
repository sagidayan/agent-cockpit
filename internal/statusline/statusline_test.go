package statusline

import (
	"testing"

	"github.com/sagidayan/agent-cockpit/internal/state"
	"github.com/stretchr/testify/require"
)

func TestToStats(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      string
		want    state.Stats
		wantErr bool
	}{
		{"full", `{
			"model": {"id": "claude-opus-5-5", "display_name": "Opus 5.5"},
			"cost": {"total_cost_usd": 1.25, "total_lines_added": 156, "total_lines_removed": 23},
			"context_window": {"total_input_tokens": 15500, "total_output_tokens": 1200, "context_window_size": 200000, "used_percentage": 8, "current_usage": {"input_tokens": 8500}}
		}`, state.Stats{Model: "Opus 5.5", ContextUsed: 15500, ContextMax: 200000, CostUSD: 1.25, LinesAdded: 156, LinesRemoved: 23}, false},
		{"session start", `{
			"model": {"display_name": "Opus 5.5"},
			"cost": {"total_cost_usd": 0},
			"context_window": {"total_input_tokens": 0, "context_window_size": 1000000, "current_usage": null, "used_percentage": null}
		}`, state.Stats{Model: "Opus 5.5", ContextMax: 1000000}, false},
		{"bad json", "nope", state.Stats{}, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := toStats([]byte(tc.in))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
