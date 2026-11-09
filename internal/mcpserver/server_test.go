package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/state"
)

func TestAddToolSurvivesPanics(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ctx := context.Background()
	s := mcp.NewServer(&mcp.Implementation{Name: "test"}, nil)
	addTool(s, &mcp.Tool{Name: "boom"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		panic("kaboom")
	})
	addTool(s, &mcp.Tool{Name: "ok"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return text("fine"), nil, nil
	})
	clientT, serverT := mcp.NewInMemoryTransports()
	_, err := s.Connect(ctx, serverT, nil)
	require.NoError(t, err)
	session, err := mcp.NewClient(&mcp.Implementation{Name: "client"}, nil).Connect(ctx, clientT, nil)
	require.NoError(t, err)
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "boom"})
	require.True(t, err != nil || res.IsError)

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "ok"})
	require.NoError(t, err)
	require.Equal(t, "fine", res.Content[0].(*mcp.TextContent).Text)

	log, err := os.ReadFile(state.MCPLog())
	require.NoError(t, err)
	require.Contains(t, string(log), "panic in boom: kaboom")
}

func TestLogRotates(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := state.MCPLog()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, make([]byte, maxLog+1), 0o644))

	logf("fresh")

	old, err := os.Stat(path + ".1")
	require.NoError(t, err)
	require.EqualValues(t, maxLog+1, old.Size())
	log, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(log), "fresh")
	require.Less(t, len(log), 100)
}
