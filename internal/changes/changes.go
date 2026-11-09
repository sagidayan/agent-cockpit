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
	var all found
	for _, x := range slices.Backward(agents.Subtree(live, a)) {
		f := r.own(x)
		all = found{add(all.repos, f.repos...), add(all.loose, f.loose...), add(all.ignored, f.ignored...)}
	}
	var parts []string
	for _, repo := range all.repos {
		d, err := gitdiff.Collect(repo)
		switch {
		case err != nil:
			parts = append(parts, "# "+repo+": "+err.Error()+"\n")
		case d != "":
			parts = append(parts, d)
		}
	}
	parts = append(parts, listing("edited outside git", all.loose)...)
	parts = append(parts, listing("gitignored", all.ignored)...)
	return strings.Join(parts, "\n")
}

func listing(title string, files []string) []string {
	if len(files) == 0 {
		return nil
	}
	return []string{"# " + title + ", no diff available:\n# " + strings.Join(files, "\n# ") + "\n"}
}

func Touched(a agents.Live) []string {
	f := newResolver().own(a)
	return slices.Concat(f.repos, f.loose, f.ignored)
}

type found struct {
	repos, loose, ignored []string
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

func (r resolver) own(x agents.Live) found {
	var f found
	for _, dir := range []string{x.Agent.Cwd, x.Pane.Cwd} {
		if root, ok := r.root(dir); ok {
			f.repos = add(f.repos, root)
		} else if paths.Disposable(dir) {
			f.repos = add(f.repos, gitdiff.Nested(dir)...)
		}
	}
	for _, p := range x.Touched {
		r.place(p, p, &f)
	}
	for _, e := range transcript.Edited(x.Agent) {
		r.place(filepath.Dir(e), e, &f)
	}
	return f
}

func (r resolver) place(lookup, path string, f *found) {
	root, ok := r.root(lookup)
	switch {
	case ok && !isDir(path) && gitdiff.Ignored(root, path):
		f.repos, f.ignored = add(f.repos, root), add(f.ignored, path)
	case ok:
		f.repos = add(f.repos, root)
	case !isDir(path) && !strings.HasPrefix(path, filepath.Join(r.home, ".claude")+"/"):
		f.loose = add(f.loose, path)
	}
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
