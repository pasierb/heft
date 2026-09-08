package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// New creates the root heft command.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "heft",
		Short:         "Seamless worktree management for herdr users",
		Args:          cobra.NoArgs,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetVersionTemplate("heft {{.Version}}\n")
	root.AddCommand(newInitCommand(), newConfigureCommand(), newWorkCommand(), newVersionCommand(version))

	return root
}

func newWorkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "work <branch>",
		Short: "Create a worktree",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			cfg, err := readConfig(filepath.Join(root, ".heft.yaml"))
			if err != nil {
				return err
			}

			branch := args[0]
			if err := exec.CommandContext(cmd.Context(), "git", "-C", root, "check-ref-format", "--branch", branch).Run(); err != nil {
				return fmt.Errorf("invalid branch %q", branch)
			}
			if err := runGit(cmd, root, "fetch", "origin", cfg.baseBranch); err != nil {
				return fmt.Errorf("fetch base branch: %w", err)
			}
			path := filepath.Join(root, cfg.worktreesDir, strings.ReplaceAll(branch, "/", "_"))
			if err := runGit(cmd, root, "worktree", "add", "-b", branch, path, "FETCH_HEAD"); err != nil {
				return fmt.Errorf("create worktree: %w", err)
			}
			return nil
		},
	}
}

func runGit(cmd *cobra.Command, root string, args ...string) error {
	git := exec.CommandContext(cmd.Context(), "git", append([]string{"-C", root}, args...)...)
	git.Stdin, git.Stdout, git.Stderr = cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr()
	return git.Run()
}

func newInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize heft",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := exec.LookPath("herdr"); err != nil {
				return fmt.Errorf("herdr is not installed or not in PATH")
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), "herdr is installed"); err != nil {
				return err
			}

			root, err := projectRoot()
			if err != nil {
				return err
			}
			if _, err := os.Stat(filepath.Join(root, ".heft.yaml")); err == nil {
				return nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return configure(cmd, root)
		},
	}
}

type config struct {
	worktreesDir string
	baseBranch   string
}

func newConfigureCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Configure heft",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			return configure(cmd, root)
		},
	}
}

func projectRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find project root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func configure(cmd *cobra.Command, root string) error {
	path := filepath.Join(root, ".heft.yaml")
	cfg, err := readConfig(path)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(cmd.InOrStdin())
	if cfg.worktreesDir, err = prompt(reader, cmd.OutOrStdout(), "Worktrees directory", cfg.worktreesDir); err != nil {
		return err
	}
	if cfg.baseBranch, err = prompt(reader, cmd.OutOrStdout(), "Base branch", cfg.baseBranch); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(root, ".heft.yaml-*")
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	contents := fmt.Sprintf("worktrees_dir: %s\nbase_branch: %s\n", yamlScalar(cfg.worktreesDir), yamlScalar(cfg.baseBranch))
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := tmp.WriteString(contents); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func readConfig(path string) (config, error) {
	cfg := config{worktreesDir: ".worktrees", baseBranch: "main"}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("read config: %w", err)
	}

	values := make(map[string]string, 2)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || value == "" || (key != "worktrees_dir" && key != "base_branch") {
			return config{}, fmt.Errorf("invalid config entry %q", scanner.Text())
		}
		if _, exists := values[key]; exists {
			return config{}, fmt.Errorf("duplicate config key %q", key)
		}
		if strings.HasPrefix(value, `"`) {
			value, err = strconv.Unquote(value)
			if err != nil {
				return config{}, fmt.Errorf("invalid value for %q: %w", key, err)
			}
		} else if yamlScalar(value) != value {
			return config{}, fmt.Errorf("invalid unquoted value for %q", key)
		}
		if value == "" {
			return config{}, fmt.Errorf("empty value for %q", key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return config{}, fmt.Errorf("read config: %w", err)
	}
	if len(values) != 2 {
		return config{}, fmt.Errorf("config must contain worktrees_dir and base_branch")
	}
	return config{worktreesDir: values["worktrees_dir"], baseBranch: values["base_branch"]}, nil
}

func prompt(reader *bufio.Reader, out io.Writer, label, current string) (string, error) {
	if _, err := fmt.Fprintf(out, "%s [%s]: ", label, current); err != nil {
		return "", err
	}
	answer, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if errors.Is(err, io.EOF) {
		if _, writeErr := fmt.Fprintln(out); writeErr != nil {
			return "", writeErr
		}
	}
	if answer = strings.TrimSpace(answer); answer != "" {
		return answer, nil
	}
	return current, nil
}

func yamlScalar(value string) string {
	for _, r := range value {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._/-", r)) {
			return strconv.Quote(value)
		}
	}
	return value
}

func newVersionCommand(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the heft version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "heft %s\n", version)
			return err
		},
	}
}
