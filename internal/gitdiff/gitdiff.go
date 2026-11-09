package gitdiff

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	emptyTree   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	nestedDepth = 3
)

func Root(path string) (string, bool) {
	dir := path
	for info, err := os.Stat(dir); err != nil || !info.IsDir(); info, err = os.Stat(dir) {
		if filepath.Dir(dir) == dir {
			return "", false
		}
		dir = filepath.Dir(dir)
	}
	top, err := git(dir, "rev-parse", "--show-toplevel")
	return strings.TrimSpace(top), err == nil
}

func Ignored(root, path string) bool {
	_, err := git(root, "check-ignore", "-q", "--", path)
	return err == nil
}

func Nested(dir string) []string {
	var roots []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if p != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
			return fs.SkipDir
		}
		if _, err := os.Stat(filepath.Join(p, ".git")); err == nil {
			if root, ok := Root(p); ok {
				roots = append(roots, root)
			}
			return fs.SkipDir
		}
		if rel, _ := filepath.Rel(dir, p); strings.Count(rel, string(filepath.Separator)) >= nestedDepth-1 {
			return fs.SkipDir
		}
		return nil
	})
	return roots
}

func Collect(dir string) (string, error) {
	if _, err := git(dir, "rev-parse", "--git-dir"); err != nil {
		return "", fmt.Errorf("%s is not a git repository", dir)
	}
	branch, err := git(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		branch = "detached HEAD"
	}
	base, label := mergeBase(dir)
	diff, err := git(dir, "diff", base)
	if err != nil {
		return "", err
	}
	added, err := untracked(dir)
	if err != nil {
		return "", err
	}
	if diff == "" && added == "" {
		return "", nil
	}
	header := fmt.Sprintf("# %s\n# branch %s compared to %s, uncommitted and untracked changes included\n\n", dir, strings.TrimSpace(branch), label)
	return header + diff + added, nil
}

func mergeBase(dir string) (base, label string) {
	for _, ref := range []string{"origin/HEAD", "origin/main", "origin/master", "main", "master"} {
		name, err := git(dir, "rev-parse", "--abbrev-ref", ref)
		if err != nil {
			continue
		}
		if sha, err := git(dir, "merge-base", "HEAD", ref); err == nil {
			return strings.TrimSpace(sha), strings.TrimSpace(name)
		}
	}
	if _, err := git(dir, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return emptyTree, "nothing (no commits yet)"
	}
	return "HEAD", "HEAD"
}

func untracked(dir string) (string, error) {
	files, err := git(dir, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for f := range strings.SplitSeq(files, "\x00") {
		if f == "" {
			continue
		}
		out, err := exec.Command("git", "-C", dir, "diff", "--no-index", "--", "/dev/null", f).Output()
		var exit *exec.ExitError
		if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
			return "", err
		}
		b.Write(out)
	}
	return b.String(), nil
}

func git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w", strings.Join(args, " "), dir, err)
	}
	return string(out), nil
}
