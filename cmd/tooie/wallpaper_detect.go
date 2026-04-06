package main

import (
	"os"
	"path/filepath"
	"strings"
)

func bestWallpaperPath(home string) (string, bool) {
	if env := strings.TrimSpace(os.Getenv("TOOIE_WALLPAPER")); env != "" {
		if wall := canonicalWallpaperCandidate(env); wall != "" {
			rememberWallpaperPath(home, wall)
			return wall, true
		}
	}
	if cached := rememberedWallpaperPath(home); cached != "" {
		return cached, true
	}
	for _, cand := range []string{
		filepath.Join(home, ".termux", "background", "background_portrait.jpeg"),
		filepath.Join(home, ".termux", "background", "background.jpeg"),
	} {
		if wall := canonicalWallpaperCandidate(cand); wall != "" {
			rememberWallpaperPath(home, wall)
			return wall, true
		}
	}
	return "", false
}

func canonicalWallpaperCandidate(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	v = strings.Trim(v, `"'`)
	if strings.HasPrefix(v, "~/") {
		v = filepath.Join(homeDir, strings.TrimPrefix(v, "~/"))
	}
	v = strings.TrimPrefix(v, "file://")
	if st, err := os.Stat(v); err == nil && !st.IsDir() && st.Size() > 0 {
		return v
	}
	return ""
}

func rememberedWallpaperPath(home string) string {
	path := filepath.Join(home, ".config", "tooie", "cache", "wallpaper-path.txt")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return canonicalWallpaperCandidate(string(raw))
}

func rememberWallpaperPath(home, wall string) {
	wall = canonicalWallpaperCandidate(wall)
	if wall == "" {
		return
	}
	path := filepath.Join(home, ".config", "tooie", "cache", "wallpaper-path.txt")
	if cur := rememberedWallpaperPath(home); cur == wall {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, []byte(wall+"\n"), 0o644)
}
