package modes

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sagidayan/agent-cockpit/internal/config"
	"github.com/sagidayan/agent-cockpit/internal/paths"
)

func TestReviewNameAndDir(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, url, wantName, wantDir string
	}{
		{"github pr", "https://github.com/acme/payments-service/pull/42", "review payments-service#42", "/tmp/prs/payments-service-42"},
		{"github pr files tab", "https://github.com/acme/web/pull/7/files", "review web#7", "/tmp/prs/web-7"},
		{"gitlab mr", "https://gitlab.com/acme/web/-/merge_requests/3", "review web#3", "/tmp/prs/web-3"},
		{"repo without number", "https://github.com/acme/web", "review web", "/tmp/prs/web"},
		{"not a url", "nonsense", "review", "/tmp/prs/review"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.wantName, ReviewName(tc.url))
			require.Equal(t, tc.wantDir, ReviewDir(tc.url))
		})
	}
}

func TestTicketName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, ticket, want string
	}{
		{"jira key", "SP-1234", "bug SP-1234"},
		{"jira browse url", "https://acme.atlassian.net/browse/SP-1234", "bug SP-1234"},
		{"jira board url", "https://acme.atlassian.net/jira/software/projects/SP/boards/1?selectedIssue=SP-9", "bug SP-9"},
		{"github issue", "https://github.com/acme/web/issues/77", "bug web#77"},
		{"linear url", "https://linear.app/acme/issue/ENG-5/crash-on-save", "bug ENG-5"},
		{"gitlab issue", "https://gitlab.com/acme/web/-/issues/5", "bug web#5"},
		{"github issue without scheme", "github.com/acme/web/issues/8", "bug web#8"},
		{"host only", "https://acme.atlassian.net/", "bug"},
		{"padded key", "  SP-3 ", "bug SP-3"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, TicketName(tc.ticket))
		})
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()
	all := All([]config.Mode{{Name: "docs", Prompt: "Write docs: {{.Task}}"}, {Name: Review, Agent: "codex"}})
	mode := func(name string) config.Mode {
		m, ok := Find(all, name)
		require.True(t, ok)
		return m
	}
	cases := []struct {
		name       string
		mode       string
		spec       Spec
		wantErr    string
		wantKind   string
		wantName   string
		wantPrompt string
	}{
		{"code passes task through", Code, Spec{Kind: "claude", Dir: "/a", Task: "fix it"}, "", "claude", "", "fix it"},
		{"code needs dir", Code, Spec{Kind: "claude"}, "directory is required", "", "", ""},
		{"review needs url", Review, Spec{}, "PR URL is required", "", "", ""},
		{"review uses mode agent and url", Review, Spec{URL: "https://github.com/o/r/pull/9"}, "", "codex", "review r#9", "Review the pull request https://github.com/o/r/pull/9"},
		{"bug needs a ticket", Bug, Spec{Kind: "claude", Dir: "/a"}, "a ticket is required", "", "", ""},
		{"bug needs dir", Bug, Spec{Kind: "claude", Ticket: "SP-12"}, "directory is required", "", "", ""},
		{"bug names itself after the ticket", Bug, Spec{Kind: "claude", Dir: "/a", Ticket: "SP-12"}, "", "claude", "bug SP-12", "Fix the bug described in SP-12"},
		{"chat gets a disposable folder", Chat, Spec{Kind: "claude", Task: "what is a rune?"}, "", "claude", "chat", "what is a rune?"},
		{"custom mode", "docs", Spec{Kind: "claude", Dir: "/a", Task: "api"}, "", "claude", "", "Write docs: api"},
		{"worktree needs a repo", Code, Spec{Kind: "claude", Dir: "/a", Worktree: true}, "/a is not in a git repository", "", "", ""},
		{"worktree needs a folder mode", Chat, Spec{Kind: "claude", Worktree: true}, "worktree needs a mode that works in a folder", "", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, err := Build(mode(tc.mode), tc.spec)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantKind, r.Kind)
			require.Equal(t, tc.mode, r.Mode)
			if tc.wantName != "" {
				require.Equal(t, tc.wantName, r.Name)
			}
			require.Contains(t, r.Prompt, tc.wantPrompt)
			if tc.mode == Chat {
				t.Cleanup(func() { _ = os.RemoveAll(r.Dir) })
				require.True(t, paths.IsChat(r.Dir))
				require.DirExists(t, r.Dir)
			}
		})
	}
}
