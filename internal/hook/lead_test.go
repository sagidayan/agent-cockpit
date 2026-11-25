package hook

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGuard(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		tool    string
		command string
		lead    bool
		want    string
	}{
		{name: "edit by a lead", tool: "Edit", lead: true, want: leadEditReason},
		{name: "write by a lead", tool: "Write", lead: true, want: leadEditReason},
		{name: "opencode patch by a lead", tool: "patch", lead: true, want: leadEditReason},
		{name: "edit by a worker", tool: "Edit"},
		{name: "read by a lead", tool: "Read", lead: true},
		{name: "worker writes", tool: "Bash", command: "rm -rf build && make"},
		{name: "worker tmux kill", tool: "Bash", command: "tmux kill-server", want: "(tmux kill-server)"},
		{name: "lead tmux kill", tool: "Bash", command: "tmux kill-server", lead: true, want: "(tmux kill-server)"},
		{name: "lead reads", tool: "Bash", command: "cat go.mod | grep module && ls -la", lead: true},
		{name: "lead searches", tool: "Bash", command: `rg -n "func main" --glob '*.go' 2>/dev/null | head -20`, lead: true},
		{name: "lead git reads", tool: "Bash", command: "git -C ../api log --oneline -5 && git status && git diff HEAD~1 --stat", lead: true},
		{name: "lead git branch list", tool: "Bash", command: "git branch -a --contains abc123", lead: true},
		{name: "lead gh views", tool: "Bash", command: "gh pr view 12 --json title && gh run list -L 5", lead: true},
		{name: "lead gh api get", tool: "Bash", command: "gh api repos/o/r/pulls/1/comments --jq '.[].body'", lead: true},
		{name: "lead jira view", tool: "Bash", command: "jira issue view SP-1 --comments 5", lead: true},
		{name: "lead kubectl get", tool: "Bash", command: "kubectl --context stg-platform -n api get pods -o wide", lead: true},
		{name: "lead gcloud describe", tool: "Bash", command: "gcloud sql instances describe db --project p", lead: true},
		{name: "lead bq select", tool: "Bash", command: "bq query --use_legacy_sql=false 'SELECT 1'", lead: true},
		{name: "lead find and xargs", tool: "Bash", command: "find . -name '*.go' | xargs grep -l TODO", lead: true},
		{name: "lead cockpit list", tool: "Bash", command: "cockpit list --json | jq '.[].status'", lead: true},
		{name: "lead fd dup", tool: "Bash", command: "ls nope 2>&1 >/dev/null", lead: true},
		{name: "lead commits", tool: "Bash", command: "git commit -m x", lead: true, want: "`git commit -m x`"},
		{name: "lead pushes after a read", tool: "Bash", command: "git status && git push", lead: true, want: "`git push`"},
		{name: "lead writes a file", tool: "Bash", command: "echo hi > notes.txt", lead: true, want: "`echo hi > notes.txt`"},
		{name: "lead appends glued", tool: "Bash", command: "cat a>>b", lead: true, want: "is not one"},
		{name: "lead redirects both", tool: "Bash", command: "ls &> out.log", lead: true, want: "is not one"},
		{name: "lead removes", tool: "Bash", command: "rm -rf build", lead: true, want: "`rm -rf build`"},
		{name: "lead runs tests", tool: "Bash", command: "go test ./...", lead: true, want: "is not one"},
		{name: "lead edits with sed", tool: "Bash", command: "sed -i '' s/a/b/ f.go", lead: true, want: "is not one"},
		{name: "lead deletes with find", tool: "Bash", command: "find . -name '*.tmp' -delete", lead: true, want: "is not one"},
		{name: "lead xargs rm", tool: "Bash", command: "ls | xargs rm", lead: true, want: "is not one"},
		{name: "lead env wrapper", tool: "Bash", command: "env FOO=1 make", lead: true, want: "is not one"},
		{name: "lead gh pr create", tool: "Bash", command: "gh pr create --fill", lead: true, want: "is not one"},
		{name: "lead gh api post", tool: "Bash", command: "gh api -X POST repos/o/r/issues", lead: true, want: "is not one"},
		{name: "lead gh api field", tool: "Bash", command: "gh api graphql -f query=x", lead: true, want: "is not one"},
		{name: "lead jira move", tool: "Bash", command: "jira issue move SP-1 Done", lead: true, want: "is not one"},
		{name: "lead kubectl delete", tool: "Bash", command: "kubectl -n api delete pod x", lead: true, want: "is not one"},
		{name: "lead bq dml", tool: "Bash", command: "bq query 'DELETE FROM t WHERE true'", lead: true, want: "is not one"},
		{name: "lead curl post", tool: "Bash", command: "curl -sX POST https://x", lead: true, want: "is not one"},
		{name: "lead curl output", tool: "Bash", command: "curl -sSLo out https://x", lead: true, want: "is not one"},
		{name: "lead subshell write", tool: "Bash", command: "echo $(touch x)", lead: true, want: "`touch x`"},
		{name: "lead dynamic command", tool: "Bash", command: "$EDITOR f", lead: true, want: "is not one"},
		{name: "lead python", tool: "Bash", command: "python3 -c 'open(\"x\",\"w\")'", lead: true, want: "is not one"},
		{name: "lead quoted arrow", tool: "Bash", command: `grep -n "->" main.go`, lead: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := guard(tc.tool, tc.command, tc.lead)
			if tc.want == "" {
				require.Empty(t, got)
				return
			}
			require.Contains(t, got, tc.want)
		})
	}
}
