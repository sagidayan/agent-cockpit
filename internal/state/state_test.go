package state

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestClosed(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	start := time.Now()
	for i := range closedLimit + 5 {
		require.NoError(t, AddClosed(Closed{Name: fmt.Sprint(i), ClosedAt: start.Add(time.Duration(i) * time.Second)}))
	}
	got := LoadClosed()
	require.Len(t, got, closedLimit)
	require.Equal(t, "24", got[0].Name)
	require.Equal(t, "5", got[closedLimit-1].Name)

	require.NoError(t, RemoveClosed(got[1]))
	got = LoadClosed()
	require.Len(t, got, closedLimit-1)
	require.Equal(t, "22", got[1].Name)
}

func TestGrouped(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.False(t, LoadGrouped())
	require.NoError(t, SaveGrouped(true))
	require.True(t, LoadGrouped())
}
