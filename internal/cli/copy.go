package cli

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func copyWorktreeFiles(root, destination string, warnings io.Writer) {
	warn := func(path string, err error) {
		fmt.Fprintf(warnings, "Warning: .heftcopy %q: %v\n", path, err)
	}
	if err := checkCopyPath(root, ".heftcopy"); err != nil {
		warn(".heftcopy", err)
		return
	}
	manifest, err := os.Open(filepath.Join(root, ".heftcopy"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		warn(".heftcopy", err)
		return
	}
	defer manifest.Close()
	scanner := bufio.NewScanner(manifest)
	for scanner.Scan() {
		entry := strings.TrimSpace(scanner.Text())
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if err := checkCopyPath(root, entry); err != nil {
			warn(entry, err)
			continue
		}
		source := filepath.Join(root, entry)
		if rel, err := filepath.Rel(source, destination); err == nil && (rel == "." || filepath.IsLocal(rel)) {
			warn(entry, fmt.Errorf("source contains the destination worktree"))
			continue
		}
		err := filepath.WalkDir(source, func(path string, d fs.DirEntry, walkErr error) error {
			rel, err := filepath.Rel(root, path)
			if err == nil {
				err = walkErr
			}
			if err == nil {
				err = checkCopyPath(root, rel)
			}
			if err == nil {
				err = checkCopyPath(destination, rel)
			}
			if err == nil {
				err = copyWorktreeEntry(path, filepath.Join(destination, rel), d)
			}
			if err != nil {
				warn(path, err)
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
			}
			return nil
		})
		if err != nil {
			warn(entry, err)
		}
	}
	if err := scanner.Err(); err != nil {
		warn(".heftcopy", err)
	}
}

// Check every component so neither source nor destination traverses a symlink.
func checkCopyPath(root, path string) error {
	if !filepath.IsLocal(path) || filepath.Clean(path) == "." {
		return fmt.Errorf("expected a path within the checkout")
	}
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if part == ".." || part == ".git" {
			return fmt.Errorf("parent traversal and Git metadata are not allowed")
		}
	}
	for _, part := range strings.Split(filepath.Clean(path), string(filepath.Separator)) {
		root = filepath.Join(root, part)
		info, err := os.Lstat(root)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not supported: %s", root)
		}
	}
	return nil
}

func copyWorktreeEntry(source, destination string, entry fs.DirEntry) error {
	if entry.IsDir() {
		return os.MkdirAll(destination, 0o755)
	}
	info, err := entry.Info()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("only regular files and directories are supported")
	}
	if _, err := os.Lstat(destination); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = io.Copy(output, input)
	if err == nil {
		err = output.Chmod(info.Mode().Perm())
	}
	closeErr := output.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(destination)
	}
	return err
}
