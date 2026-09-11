package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWorkCommand() *cobra.Command {
	var prompt string
	var profileName string
	var noFocus bool
	var label string
	cmd := &cobra.Command{
		Use:   "work <branch>",
		Short: "Create a task worktree and workspace",
		Long: `Create or reuse a branch, check it out in the configured worktree directory,
and open a matching Herdr workspace.

New branches start from origin/<base_branch>. Existing local or remote branches
and matching worktrees at the configured path are reused. Each invocation opens
a new workspace. Use --prompt to send work directly to a configured agent tab.

New worktrees receive files listed in the primary checkout's .heftcopy before
the workspace opens. Existing files are preserved; copy errors warn and continue.`,
		Example: `  heft work feature/login
  heft work fizzy-40 --prompt "Analyze card 40 and implement it"
  heft work bugfix/session --profile research --no-focus`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			configPath := filepath.Join(root, ".heft.yaml")
			cfg, err := readConfig(configPath)
			if err != nil {
				return err
			}
			if profileName != "" {
				profile, ok := cfg.Profiles[profileName]
				if !ok {
					return fmt.Errorf("profile %q is not configured in %s", profileName, configPath)
				}
				cfg.Tabs = profile.Tabs
			}
			hasAgent := false
			for _, tab := range cfg.Tabs {
				hasAgent = hasAgent || tab.Agent
			}
			if prompt != "" && !hasAgent {
				return fmt.Errorf("--prompt requires a configured agent tab")
			}
			if err := ensureWorktreesIgnored(root, cfg.WorktreesDir); err != nil {
				return err
			}
			branch := args[0]
			if label == "" {
				prefix := strings.TrimSpace(cfg.WorkspacePrefix)
				if prefix == "" {
					prefix, err = repositoryName(root)
					if err != nil {
						return err
					}
				}
				label = prefix + " " + branch
			}
			if err := validateBranch(cmd, root, branch); err != nil {
				return err
			}
			if err := runGit(cmd, root, "fetch", "origin"); err != nil {
				return fmt.Errorf("fetch base branch: %w", err)
			}
			path := worktreePath(root, cfg.WorktreesDir, branch)
			git := exec.CommandContext(cmd.Context(), "git", "-C", root, "worktree", "list", "--porcelain", "-z")
			git.Stderr = cmd.ErrOrStderr()
			out, err := git.Output()
			if err != nil {
				return fmt.Errorf("list worktrees: %w", err)
			}
			reused := false
			for _, record := range strings.Split(string(out), "\x00\x00") {
				if !strings.HasPrefix(record, "worktree "+path+"\x00") {
					continue
				}
				if !slices.Contains(strings.Split(record, "\x00"), "branch refs/heads/"+branch) {
					return fmt.Errorf("worktree at %s is not on branch %q", path, branch)
				}
				if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
					return fmt.Errorf("check existing worktree at %s: %w", path, err)
				}
				reused = true
				break
			}
			if reused {
				fmt.Fprintf(cmd.OutOrStdout(), "Reusing worktree at %s.\n", path)
			} else {
				gitArgs := []string{"worktree", "add", "-b", branch, path, "origin/" + cfg.BaseBranch}
				existed := ""
				if gitRefExists(cmd, root, "refs/heads/"+branch) {
					gitArgs = []string{"worktree", "add", path, branch}
					existed = "locally"
				} else if gitRefExists(cmd, root, "refs/remotes/origin/"+branch) {
					gitArgs = []string{"worktree", "add", "--track", "-b", branch, path, "origin/" + branch}
					existed = "on origin"
				}
				if err := runGit(cmd, root, gitArgs...); err != nil {
					return fmt.Errorf("create worktree: %w", err)
				}
				if existed != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Branch %q already exists %s; checking it out.\n", branch, existed)
				}
				copyWorktreeFiles(root, path, cmd.ErrOrStderr())
			}
			if len(cfg.Tabs) == 0 {
				args := []string{"workspace", "create", "--cwd", path, "--label", label}
				if noFocus {
					args = append(args, "--no-focus")
				}
				return runHerdr(cmd, nil, "create herdr workspace", args...)
			}
			focus := "--focus"
			if noFocus {
				focus = "--no-focus"
			}
			created, err := createHerdr(cmd, "create herdr workspace", "workspace", "create", "--cwd", path, "--label", label, focus)
			if err != nil {
				return err
			}
			if created.Result.Workspace.WorkspaceID == "" || created.Result.Tab.TabID == "" || created.Result.RootPane.PaneID == "" {
				return fmt.Errorf("create herdr workspace: response missing resource IDs")
			}
			if err := runHerdr(cmd, nil, "rename first herdr tab", "tab", "rename", created.Result.Tab.TabID, cfg.Tabs[0].Name); err != nil {
				return err
			}
			if cfg.Tabs[0].Command != "" {
				if err := runHerdr(cmd, nil, "run command in first herdr tab", "pane", "run", created.Result.RootPane.PaneID, cfg.Tabs[0].Command); err != nil {
					return err
				}
			}
			workspaceID := created.Result.Workspace.WorkspaceID
			agentPaneID := ""
			if cfg.Tabs[0].Agent {
				agentPaneID = created.Result.RootPane.PaneID
			}
			for _, tab := range cfg.Tabs[1:] {
				created, err := createHerdr(cmd, fmt.Sprintf("create herdr tab %q", tab.Name), "tab", "create", "--workspace", workspaceID, "--cwd", path, "--label", tab.Name, "--no-focus")
				if err != nil {
					return err
				}
				if created.Result.RootPane.PaneID == "" {
					return fmt.Errorf("create herdr tab %q: response missing root pane ID", tab.Name)
				}
				if tab.Command != "" {
					if err := runHerdr(cmd, nil, fmt.Sprintf("run command in herdr tab %q", tab.Name), "pane", "run", created.Result.RootPane.PaneID, tab.Command); err != nil {
						return err
					}
				}
				if tab.Agent {
					agentPaneID = created.Result.RootPane.PaneID
				}
			}
			if prompt != "" {
				return promptAgent(cmd, agentPaneID, prompt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "send a prompt to the configured agent tab")
	cmd.Flags().StringVar(&profileName, "profile", "", "use the named tab profile from .heft.yaml")
	cmd.Flags().BoolVar(&noFocus, "no-focus", false, "create the workspace without focusing it")
	cmd.Flags().StringVar(&label, "label", "", "override the Herdr workspace label")
	return cmd
}

func gitRefExists(cmd *cobra.Command, root, ref string) bool {
	return exec.CommandContext(cmd.Context(), "git", "-C", root, "show-ref", "--verify", "--quiet", ref).Run() == nil
}

func newListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List worktrees",
		Long:    "List all worktrees registered with the current Git repository.",
		Example: "  heft list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := projectRoot()
			if err != nil {
				return err
			}
			return runGit(cmd, root, "worktree", "list")
		},
	}
}

type herdrCreation struct {
	Result struct {
		Workspace struct {
			WorkspaceID string `json:"workspace_id"`
		} `json:"workspace"`
		Tab struct {
			TabID string `json:"tab_id"`
		} `json:"tab"`
		RootPane struct {
			PaneID string `json:"pane_id"`
		} `json:"root_pane"`
	} `json:"result"`
}

func createHerdr(cmd *cobra.Command, action string, args ...string) (herdrCreation, error) {
	var created herdrCreation
	var out bytes.Buffer
	if err := runHerdr(cmd, &out, action, args...); err != nil {
		return created, err
	}
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		return created, fmt.Errorf("%s: decode response: %w", action, err)
	}
	return created, nil
}

func runHerdr(cmd *cobra.Command, out *bytes.Buffer, action string, args ...string) error {
	herdr := exec.CommandContext(cmd.Context(), "herdr", args...)
	herdr.Stdin, herdr.Stderr = cmd.InOrStdin(), cmd.ErrOrStderr()
	if out == nil {
		herdr.Stdout = cmd.OutOrStdout()
	} else {
		herdr.Stdout = out
	}
	if err := herdr.Run(); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func promptAgent(cmd *cobra.Command, paneID, prompt string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		probe := exec.CommandContext(cmd.Context(), "herdr", "agent", "get", paneID)
		if err := probe.Run(); err == nil {
			break
		} else if time.Now().After(deadline) {
			return fmt.Errorf("wait for agent in configured herdr tab: %w", err)
		}
		select {
		case <-cmd.Context().Done():
			return fmt.Errorf("wait for agent in configured herdr tab: %w", cmd.Context().Err())
		case <-time.After(100 * time.Millisecond):
		}
	}

	remaining := max(time.Until(deadline).Milliseconds(), 1)
	var out bytes.Buffer
	if err := runHerdr(cmd, &out, "wait for agent in configured herdr tab", "agent", "wait", paneID, "--timeout", strconv.FormatInt(remaining, 10)); err != nil {
		return err
	}
	return runHerdr(cmd, &out, "prompt agent in configured herdr tab", "agent", "prompt", paneID, prompt)
}
