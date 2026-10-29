// Package config loads gwm's user configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const defaultWorktreesRoot = ".worktrees"

// Config is the content of config.json.
type Config struct {
	WorktreesRoot string                   `json:"worktreesRoot"`
	Projects      map[string]ProjectConfig `json:"projects"`
}

// ProjectConfig overrides Config for one Project, keyed by its main worktree path.
type ProjectConfig struct {
	WorktreesRoot string `json:"worktreesRoot"`
}

// Path returns the config file location:
// $XDG_CONFIG_HOME/worktree-manager/config.json, or ~/.config/... when unset.
func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "worktree-manager", "config.json")
}

// Load reads the config at path. A missing file yields the defaults.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// WorktreesRootFor returns the absolute Worktrees root for the Project at mainPath.
// Relative paths are resolved against mainPath; a leading ~ is the home directory.
func (c Config) WorktreesRootFor(mainPath string) string {
	root := c.WorktreesRoot
	if p, ok := c.Projects[mainPath]; ok && p.WorktreesRoot != "" {
		root = p.WorktreesRoot
	}
	if root == "" {
		root = defaultWorktreesRoot
	}
	if root == "~" || strings.HasPrefix(root, "~/") {
		root = filepath.Join(os.Getenv("HOME"), root[1:])
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(mainPath, root)
	}
	return filepath.Clean(root)
}
