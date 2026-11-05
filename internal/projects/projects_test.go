package projects

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForDir(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	list := []Project{{Name: "api", Dir: "~/src/api"}, {Name: "web", Dir: "/opt/web"}}
	cases := []struct {
		name string
		dir  string
		want string
	}{
		{"tilde dir", home + "/src/api", "api"},
		{"absolute dir", "/opt/web", "web"},
		{"subdir is not a match", "/opt/web/src", ""},
		{"unknown", "/tmp", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, ok := ForDir(list, tc.dir)
			require.Equal(t, tc.want != "", ok)
			require.Equal(t, tc.want, p.Name)
		})
	}
}
