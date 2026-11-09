package tmux

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePanes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		out  string
		want []string
	}{
		{"empty trailing label kept", "%0\tw\t0\tapi\t/a\tclaude\t\t\t\t12\n%1\tw\t1\tweb\t/b\tclaude\t\t\t\t12\n", []string{"%0", "%1"}},
		{"label with spaces", "%2\tw\t2\tapi\t/a\tclaude\tbilling bug\treview\t%1\t14\n", []string{"%2"}},
		{"linked window listed once", "%4\tmain\t1\tapi\t/a\tclaude\t\t\t\t12\n%4\tcockpit\t3\tapi\t/a\tclaude\t\t\t\t12\n", []string{"%4"}},
		{"malformed line skipped", "garbage\n%3\tw\t3\tapi\t/a\tzsh\t\t\t\t13\n", []string{"%3"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ids []string
			for _, p := range parsePanes(tc.out) {
				ids = append(ids, p.ID)
			}
			require.Equal(t, tc.want, ids)
		})
	}
}
