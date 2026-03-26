package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func resolveHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return "/data/data/com.termux/files/home"
}

func ensureTooieSupportScripts() error {
	scripts := []struct {
		path    string
		repoRel string
	}{
		{path: installedApplyScriptPath(), repoRel: filepath.Join("scripts", "apply-material.sh")},
		{path: installedRestoreScriptPath(), repoRel: filepath.Join("scripts", "restore-material.sh")},
		{path: filepath.Join(tooieConfigDir, "list-material-backups.sh"), repoRel: filepath.Join("scripts", "list-material-backups.sh")},
		{path: installedResetScriptPath(), repoRel: filepath.Join("scripts", "reset-bootstrap-defaults.sh")},
		{path: installedBtopSetupScriptPath(), repoRel: filepath.Join("scripts", "setup-btop-helper.sh")},
	}
	for _, script := range scripts {
		if err := ensureSupportScript(script.path, script.repoRel); err != nil {
			return err
		}
	}
	return nil
}

func ensureSupportScript(path, repoRel string) error {
	if repoPath := resolveRepoSupportPath(repoRel); repoPath != "" {
		raw, err := os.ReadFile(repoPath)
		if err != nil {
			return err
		}
		return writeSupportScript(path, raw)
	}

	info, err := os.Stat(path)
	if err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
		return nil
	}
	return fmt.Errorf("required support script missing: %s (run ./install.sh)", path)
}

func writeSupportScript(path string, raw []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil {
		info, statErr := os.Stat(path)
		if statErr == nil && !info.IsDir() && info.Mode()&0o111 != 0 && bytesEqualTrimmed(existing, raw) {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o755); err != nil {
		return err
	}
	return nil
}

func installedApplyScriptPath() string {
	return filepath.Join(tooieConfigDir, "apply-material.sh")
}

func installedRestoreScriptPath() string {
	return filepath.Join(tooieConfigDir, "restore-material.sh")
}

func currentApplyScriptPath() string {
	return installedApplyScriptPath()
}

func currentRestoreScriptPath() string {
	return installedRestoreScriptPath()
}

func installedResetScriptPath() string {
	return filepath.Join(tooieConfigDir, "reset-bootstrap-defaults.sh")
}

func currentResetScriptPath() string {
	return installedResetScriptPath()
}

func installedBtopSetupScriptPath() string {
	return filepath.Join(tooieConfigDir, "setup-btop-helper.sh")
}

func currentBtopSetupScriptPath() string {
	return installedBtopSetupScriptPath()
}

func resolveRepoSupportPath(repoRel string) string {
	candidates := []string{}
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, repoRel))
	}
	if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
		candidates = append(candidates, filepath.Join(wd, repoRel))
	}
	candidates = append(candidates, filepath.Join(homeDir, "files", "tooie", repoRel))
	for _, p := range candidates {
		info, err := os.Stat(p)
		if err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func bytesEqualTrimmed(a, b []byte) bool {
	return strings.TrimSpace(string(a)) == strings.TrimSpace(string(b))
}
