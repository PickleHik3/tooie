package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type installSnapshotEntry struct {
	Path      string `json:"path"`
	Kind      string `json:"kind"`
	Existed   bool   `json:"existed"`
	BackupRel string `json:"backup_rel,omitempty"`
}

type installSnapshotManifest struct {
	Version   int                    `json:"version"`
	CreatedAt string                 `json:"created_at"`
	HomeDir   string                 `json:"home_dir"`
	Snapshot  string                 `json:"snapshot"`
	Entries   []installSnapshotEntry `json:"entries"`
}

func installStateRoot() string {
	return filepath.Join(homeDir, ".local", "state", "tooie", "install")
}

func installSnapshotsRoot() string {
	return filepath.Join(installStateRoot(), "snapshots")
}

func installLatestPath() string {
	return filepath.Join(installStateRoot(), "latest.json")
}

func captureInstallSnapshot(paths []string) (string, error) {
	id := time.Now().Format("20060102-150405")
	snapDir := filepath.Join(installSnapshotsRoot(), id)
	filesDir := filepath.Join(snapDir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return "", err
	}
	entries := make([]installSnapshotEntry, 0, len(paths))
	for i, p := range paths {
		abs := strings.TrimSpace(p)
		if abs == "" {
			continue
		}
		ent := installSnapshotEntry{Path: abs}
		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				ent.Existed = false
				ent.Kind = "missing"
				entries = append(entries, ent)
				continue
			}
			return "", err
		}
		ent.Existed = true
		kind := "file"
		if info.IsDir() {
			kind = "dir"
		}
		ent.Kind = kind
		backupRel := fmt.Sprintf("%03d-%s", i+1, kind)
		ent.BackupRel = filepath.Join("files", backupRel)
		backupPath := filepath.Join(snapDir, ent.BackupRel)
		if info.IsDir() {
			if err := copyDirAll(abs, backupPath); err != nil {
				return "", err
			}
		} else {
			if err := copyFileMode(abs, backupPath, info.Mode().Perm()); err != nil {
				return "", err
			}
		}
		entries = append(entries, ent)
	}
	manifest := installSnapshotManifest{
		Version:   1,
		CreatedAt: time.Now().Format(time.RFC3339),
		HomeDir:   homeDir,
		Snapshot:  id,
		Entries:   entries,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(snapDir, "manifest.json"), raw, 0o644); err != nil {
		return "", err
	}
	if err := os.MkdirAll(installStateRoot(), 0o755); err != nil {
		return "", err
	}
	latest := map[string]string{
		"snapshot":   id,
		"manifest":   filepath.Join(snapDir, "manifest.json"),
		"created_at": manifest.CreatedAt,
	}
	lraw, err := json.MarshalIndent(latest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(installLatestPath(), lraw, 0o644); err != nil {
		return "", err
	}
	return id, nil
}

func latestInstallSnapshotID() (string, error) {
	raw, err := os.ReadFile(installLatestPath())
	if err != nil {
		return "", err
	}
	var v map[string]string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	return strings.TrimSpace(v["snapshot"]), nil
}

func latestSnapshotBackupForPath(target string) (string, error) {
	snapID, err := latestInstallSnapshotID()
	if err != nil {
		return "", err
	}
	snapID = strings.TrimSpace(snapID)
	if snapID == "" {
		return "", fmt.Errorf("latest snapshot id is empty")
	}
	manifestPath := filepath.Join(installSnapshotsRoot(), snapID, "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", err
	}
	var manifest installSnapshotManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return "", err
	}
	want := strings.TrimSpace(target)
	for _, ent := range manifest.Entries {
		if strings.TrimSpace(ent.Path) != want {
			continue
		}
		if !ent.Existed || strings.TrimSpace(ent.BackupRel) == "" {
			return "", fmt.Errorf("no backed up entry for %s", target)
		}
		backup := filepath.Join(installSnapshotsRoot(), snapID, ent.BackupRel)
		if _, err := os.Stat(backup); err != nil {
			return "", err
		}
		return backup, nil
	}
	return "", fmt.Errorf("path not present in snapshot: %s", target)
}

func restoreInstallSnapshot(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		var err error
		id, err = latestInstallSnapshotID()
		if err != nil {
			return fmt.Errorf("unable to resolve latest snapshot: %w", err)
		}
	}
	manifestPath := filepath.Join(installSnapshotsRoot(), id, "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest installSnapshotManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return err
	}
	snapDir := filepath.Join(installSnapshotsRoot(), id)
	for _, ent := range manifest.Entries {
		target := strings.TrimSpace(ent.Path)
		if target == "" {
			continue
		}
		if !ent.Existed {
			_ = os.RemoveAll(target)
			continue
		}
		src := filepath.Join(snapDir, ent.BackupRel)
		info, err := os.Stat(src)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(target); err != nil {
			return err
		}
		if info.IsDir() {
			if err := copyDirAll(src, target); err != nil {
				return err
			}
		} else {
			if err := copyFileMode(src, target, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFileMode(src, dst string, perm os.FileMode) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, raw, perm)
}

func copyDirAll(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFileMode(path, target, info.Mode().Perm())
	})
}
