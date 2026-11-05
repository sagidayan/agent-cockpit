package state

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateSavesOnce(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("TMUX", "/tmp/sock,1,0")
	require.NoError(t, Save(Agent{Pane: "%1", Kind: "claude", Status: Working}))

	var saves atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			_, _, saved, err := Update("%1", func(prev Agent) (Agent, bool) {
				if prev.Status != Working {
					return prev, false
				}
				prev.Status = Done
				return prev, true
			})
			require.NoError(t, err)
			if saved {
				saves.Add(1)
			}
		})
	}
	wg.Wait()

	require.Equal(t, int32(1), saves.Load())
	got, err := Load("%1")
	require.NoError(t, err)
	require.Equal(t, Done, got.Status)
}
