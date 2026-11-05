package gitdiff

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

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
