package worktree

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const branchPrefix = "cockpit/"

func InRepo(dir string) bool {
	_, err := git(dir, "rev-parse", "--show-toplevel")
	return err == nil
}

func Create(dir, name string) (string, error) {
	root, err := mainRoot(dir)
	if err != nil {
		return "", fmt.Errorf("%s is not in a git repository", dir)
	}
	parent := root + "-worktrees"
	path, branch, err := free(dir, parent, slugify(name))
	if err != nil {
		return "", err
	}
	if _, err := git(dir, "worktree", "add", "-q", "-b", branch, path, "HEAD"); err != nil {
		return "", err
	}
	return path, nil
}

func mainRoot(dir string) (string, error) {
	out, err := git(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(out, "\n")
	root, ok := strings.CutPrefix(first, "worktree ")
	if !ok {
		return "", fmt.Errorf("unexpected git worktree list output: %q", first)
	}
	return filepath.Clean(root), nil
}

func free(dir, parent, slug string) (path, branch string, err error) {
	for n := 1; n <= 100; n++ {
		name := slug
		if n > 1 {
			name += "-" + strconv.Itoa(n)
		}
		path, branch = filepath.Join(parent, name), branchPrefix+name
		if !exists(path) && !hasBranch(dir, branch) {
			return path, branch, nil
		}
	}
	return "", "", fmt.Errorf("no free worktree name for %q in %s", slug, parent)
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func hasBranch(dir, branch string) bool {
	_, err := git(dir, "rev-parse", "--verify", "-q", "refs/heads/"+branch)
	return err == nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	if s == "" {
		return "agent"
	}
	return s
}

func git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exit.Stderr)))
	}
	if err != nil {
		return "", err
	}
	return string(out), nil
}
