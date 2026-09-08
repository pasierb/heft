package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

type config struct {
	WorktreesDir    string             `yaml:"worktrees_dir"`
	BaseBranch      string             `yaml:"base_branch"`
	WorkspacePrefix string             `yaml:"workspace_prefix,omitempty"`
	Tabs            []tab              `yaml:"tabs,omitempty"`
	Profiles        map[string]profile `yaml:"profiles,omitempty"`
	Extra           map[string]any     `yaml:",inline"`
}

type profile struct {
	Tabs  []tab          `yaml:"tabs,omitempty"`
	Extra map[string]any `yaml:",inline"`
}

type tab struct {
	Name    string         `yaml:"name"`
	Command string         `yaml:"command,omitempty"`
	Agent   bool           `yaml:"agent,omitempty"`
	Extra   map[string]any `yaml:",inline"`
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
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	if cfg.WorktreesDir == "" {
		cfg.WorktreesDir = ".worktrees"
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = "main"
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	if cfg.WorktreesDir, err = prompt(reader, cmd.OutOrStdout(), "Worktrees directory", cfg.WorktreesDir); err != nil {
		return err
	}
	if cfg.BaseBranch, err = prompt(reader, cmd.OutOrStdout(), "Base branch", cfg.BaseBranch); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.WorkspacePrefix) == "" {
		if cfg.WorkspacePrefix, err = repositoryName(root); err != nil {
			return err
		}
	}
	if cfg.WorkspacePrefix, err = prompt(reader, cmd.OutOrStdout(), "Workspace prefix", cfg.WorkspacePrefix); err != nil {
		return err
	}
	cfg.WorkspacePrefix = strings.TrimSpace(cfg.WorkspacePrefix)
	if !hasAgentTab(cfg.Tabs) {
		agent, err := promptHarness(reader, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		cfg.Tabs = append([]tab{agent}, cfg.Tabs...)
	}
	if err := validateConfig(cfg); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(root, ".heft.yaml-*")
	if err != nil {
		return fmt.Errorf("create config: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	contents, err := yaml.Marshal(cfg)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("encode config: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := tmp.Write(contents); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return ensureWorktreesIgnored(root, cfg.WorktreesDir)
}

func hasAgentTab(tabs []tab) bool {
	for _, tab := range tabs {
		if tab.Agent {
			return true
		}
	}
	return false
}

func promptHarness(reader *bufio.Reader, out io.Writer) (tab, error) {
	if _, err := fmt.Fprintln(out, "Default harness:\n  1) Claude\n  2) Codex\n  3) Agy\n  4) Other"); err != nil {
		return tab{}, err
	}
	for {
		if _, err := fmt.Fprint(out, "Select [1-4]: "); err != nil {
			return tab{}, err
		}
		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return tab{}, err
		}
		switch strings.TrimSpace(answer) {
		case "1":
			return tab{Name: "claude", Command: "claude", Agent: true}, nil
		case "2":
			return tab{Name: "codex", Command: "codex", Agent: true}, nil
		case "3":
			return tab{Name: "agy", Command: "agy", Agent: true}, nil
		case "4":
			command, err := promptRequired(reader, out, "Harness command")
			if err != nil {
				return tab{}, err
			}
			return tab{Name: filepath.Base(strings.Fields(command)[0]), Command: command, Agent: true}, nil
		}
		if errors.Is(err, io.EOF) {
			return tab{}, fmt.Errorf("configure default harness: input ended before a valid selection")
		}
	}
}

func promptRequired(reader *bufio.Reader, out io.Writer, label string) (string, error) {
	for {
		if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
			return "", err
		}
		answer, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		if answer = strings.TrimSpace(answer); answer != "" {
			return answer, nil
		}
		if errors.Is(err, io.EOF) {
			return "", fmt.Errorf("configure default harness: input ended before a command was provided")
		}
	}
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
	cfg, err := loadConfig(path)
	if err != nil {
		return config{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return config{}, err
	}
	return cfg, nil
}

func loadConfig(path string) (config, error) {
	cfg := config{WorktreesDir: ".worktrees", BaseBranch: "main", Tabs: []tab{{Name: "shell"}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return config{}, fmt.Errorf("read config: %w", err)
	}
	cfg = config{}
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&cfg); err != nil {
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return config{}, fmt.Errorf("parse config: multiple YAML documents")
		}
		return config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

func validateConfig(cfg config) error {
	if cfg.WorktreesDir == "" || cfg.BaseBranch == "" {
		return fmt.Errorf("config must contain worktrees_dir and base_branch; run heft configure")
	}
	if err := validateTabs("tabs", cfg.Tabs); err != nil {
		return err
	}
	for name, profile := range cfg.Profiles {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("profiles must not contain an empty name")
		}
		if err := validateTabs(fmt.Sprintf("profiles[%q].tabs", name), profile.Tabs); err != nil {
			return err
		}
	}
	return nil
}

func validateTabs(path string, tabs []tab) error {
	agentTab := -1
	for i, tab := range tabs {
		if strings.TrimSpace(tab.Name) == "" {
			return fmt.Errorf("%s[%d].name must not be empty", path, i)
		}
		if tab.Agent {
			if agentTab >= 0 {
				return fmt.Errorf("%s[%d].agent: only one agent tab may be configured", path, i)
			}
			if strings.TrimSpace(tab.Command) == "" {
				return fmt.Errorf("%s[%d].agent requires command", path, i)
			}
			agentTab = i
		}
	}
	return nil
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
