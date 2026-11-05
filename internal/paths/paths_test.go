package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsChat(t *testing.T) {
	t.Parallel()
	chat, err := NewChat()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(chat) })
	cases := []struct {
		name string
		dir  string
		want bool
	}{
		{"new chat", chat, true},
		{"chat root itself", filepath.Dir(chat), false},
		{"inside a chat", filepath.Join(chat, "sub"), false},
		{"review folder", "/tmp/prs/web-7", false},
		{"home", Expand("~"), false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, IsChat(tc.dir))
		})
	}
}
