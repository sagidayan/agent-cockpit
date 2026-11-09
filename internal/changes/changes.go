package changes

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/gitdiff"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/transcript"
)

func Diff(a agents.Live, live []agents.Live) string {
	r := newResolver()
	var repos, loose []string
	for _, x := range slices.Backward(agents.Subtree(live, a)) {
		rs, ls := r.own(x)
		repos, loose = add(repos, rs...), add(loose, ls...)
	}
	var parts []string
	for _, repo := range repos {
		d, err := gitdiff.Collect(repo)
		switch {
		case err != nil:
			parts = append(parts, "# "+repo+": "+err.Error()+"\n")
		case d != "":
			parts = append(parts, d)
		}
	}
	if len(loose) > 0 {
		parts = append(parts, "# edited outside git, no diff available:\n# "+strings.Join(loose, "\n# ")+"\n")
	}
	return strings.Join(parts, "\n")
}

func Touched(a agents.Live) []string {
	repos, loose := newResolver().own(a)
	return append(repos, loose...)
}

type resolver struct {
	home  string
	roots map[string]string
}

func newResolver() resolver {
	home, _ := os.UserHomeDir()
	return resolver{home, map[string]string{}}
}

func (r resolver) root(path string) (string, bool) {
	root, ok := r.roots[path]
	if !ok {
		root, _ = gitdiff.Root(path)
		if root == r.home {
			root = ""
		}
		r.roots[path] = root
	}
	return root, root != ""
}

func (r resolver) own(x agents.Live) (repos, loose []string) {
	for _, dir := range []string{x.Agent.Cwd, x.Pane.Cwd} {
		if root, ok := r.root(dir); ok {
			repos = add(repos, root)
		} else if paths.Disposable(dir) {
			repos = add(repos, gitdiff.Nested(dir)...)
		}
	}
	for _, p := range x.Touched {
		repos, loose = r.place(p, p, repos, loose)
	}
	for _, f := range transcript.Edited(x.Agent) {
		repos, loose = r.place(filepath.Dir(f), f, repos, loose)
	}
	return repos, loose
}

func (r resolver) place(lookup, path string, repos, loose []string) ([]string, []string) {
	if root, ok := r.root(lookup); ok {
		return add(repos, root), loose
	}
	if isDir(path) || strings.HasPrefix(path, filepath.Join(r.home, ".claude")+"/") {
		return repos, loose
	}
	return repos, add(loose, path)
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func add(list []string, items ...string) []string {
	for _, it := range items {
		if it != "" && !slices.Contains(list, it) {
			list = append(list, it)
		}
	}
	return list
}
