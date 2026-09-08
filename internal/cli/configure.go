package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

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
	return ensureWorktreesIgnored(root, cfg.worktreesDir)
}

func ensureWorktreesIgnored(root, dir string) error {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read .gitignore: %w", err)
	}
	entry := "/" + strings.Trim(filepath.ToSlash(filepath.Clean(dir)), "/") + "/"
	for _, line := range strings.Split(string(data), "\n") {
		if line == entry {
			return nil
		}
	}
	prefix := ""
	if len(data) > 0 && data[len(data)-1] != '\n' {
		prefix = "\n"
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	if _, err := fmt.Fprintf(file, "%s%s\n", prefix, entry); err != nil {
		file.Close()
		return fmt.Errorf("write .gitignore: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
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
