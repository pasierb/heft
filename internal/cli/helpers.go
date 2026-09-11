package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

func projectRoot() (string, error) {
	out, err := exec.Command("git", "worktree", "list", "--porcelain", "-z").Output()
	if err != nil {
		return "", fmt.Errorf("find project root: %w", err)
	}
	record, _, _ := strings.Cut(string(out), "\x00\x00")
	fields := strings.Split(record, "\x00")
	root, ok := strings.CutPrefix(fields[0], "worktree ")
	if !ok || !filepath.IsAbs(root) || slices.Contains(fields, "bare") {
		return "", fmt.Errorf("find project root: repository has no primary checkout")
	}
	// Submodules list their Git metadata directory rather than their checkout.
	out, err = exec.Command("git", "-C", root, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find project root: resolve primary checkout: %w", err)
	}
	root = strings.TrimSuffix(string(out), "\n")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("find project root: primary checkout path is not absolute: %q", root)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("find project root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("find project root: %s is not a directory", root)
	}
	return root, nil
}

func repositoryName(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("find repository name: %w", err)
	}
	return filepath.Base(filepath.Dir(strings.TrimSpace(string(out)))), nil
}

func runGit(cmd *cobra.Command, root string, args ...string) error {
	git := exec.CommandContext(cmd.Context(), "git", append([]string{"-C", root}, args...)...)
	git.Stdin, git.Stdout, git.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return git.Run()
}

func validateBranch(cmd *cobra.Command, root, branch string) error {
	if err := exec.CommandContext(cmd.Context(), "git", "-C", root, "check-ref-format", "--branch", branch).Run(); err != nil {
		return fmt.Errorf("invalid branch %q", branch)
	}
	return nil
}

func worktreePath(root, dir, branch string) string {
	return filepath.Join(root, dir, strings.ReplaceAll(branch, "/", "_"))
}
