package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func bestWallpaperPath(home string) (string, bool) {
	candidates := []string{}
	if env := strings.TrimSpace(os.Getenv("TOOIE_WALLPAPER")); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates,
		filepath.Join(home, ".termux", "background", "background_portrait.jpeg"),
		filepath.Join(home, ".termux", "background", "background.jpeg"),
	)
	if gnome := detectGNOMEWallpaper(); gnome != "" {
		candidates = append(candidates, gnome)
	}
	if xfce := detectXFCEWallpaper(); xfce != "" {
		candidates = append(candidates, xfce)
	}
	if kde := detectKDEWallpaper(home); kde != "" {
		candidates = append(candidates, kde)
	}
	if hypr := detectHyprpaperWallpaper(home); hypr != "" {
		candidates = append(candidates, hypr)
	}
	if sway := detectSwayWallpaper(home); sway != "" {
		candidates = append(candidates, sway)
	}
	if feh := detectFehWallpaper(home); feh != "" {
		candidates = append(candidates, feh)
	}
	if latest := newestImageFromCommonDirs(home); latest != "" {
		candidates = append(candidates, latest)
	}
	if latest := newestImageFromPicturesFiltered(home); latest != "" {
		candidates = append(candidates, latest)
	}
	for _, c := range candidates {
		if wall := canonicalWallpaperCandidate(c); wall != "" {
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
	v = strings.TrimPrefix(v, "file://")
	if st, err := os.Stat(v); err == nil && !st.IsDir() && st.Size() > 0 {
		return v
	}
	return ""
}

func detectGNOMEWallpaper() string {
	if _, err := exec.LookPath("gsettings"); err != nil {
		return ""
	}
	keys := []string{
		"org.gnome.desktop.background picture-uri-dark",
		"org.gnome.desktop.background picture-uri",
		"org.cinnamon.desktop.background picture-uri",
		"org.mate.background picture-filename",
	}
	for _, key := range keys {
		parts := strings.Split(key, " ")
		out, err := exec.Command("gsettings", "get", parts[0], parts[1]).CombinedOutput()
		if err != nil {
			continue
		}
		if wall := canonicalWallpaperCandidate(string(out)); wall != "" {
			return wall
		}
	}
	return ""
}

func detectXFCEWallpaper() string {
	if _, err := exec.LookPath("xfconf-query"); err != nil {
		return ""
	}
	keys := []string{
		"/backdrop/screen0/monitor0/image-path",
		"/backdrop/screen0/monitor0/workspace0/last-image",
	}
	for _, key := range keys {
		out, err := exec.Command("xfconf-query", "-c", "xfce4-desktop", "-p", key).CombinedOutput()
		if err != nil {
			continue
		}
		if wall := canonicalWallpaperCandidate(string(out)); wall != "" {
			return wall
		}
	}
	return ""
}

func detectKDEWallpaper(home string) string {
	path := filepath.Join(home, ".config", "plasma-org.kde.plasma.desktop-appletsrc")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Image=") {
			continue
		}
		if wall := canonicalWallpaperCandidate(strings.TrimPrefix(line, "Image=")); wall != "" {
			return wall
		}
	}
	return ""
}

func detectHyprpaperWallpaper(home string) string {
	path := filepath.Join(home, ".config", "hypr", "hyprpaper.conf")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "wallpaper") || !strings.Contains(line, "=") {
			continue
		}
		rhs := strings.TrimSpace(strings.SplitN(line, "=", 2)[1])
		if strings.Contains(rhs, ",") {
			rhs = strings.TrimSpace(strings.Split(rhs, ",")[len(strings.Split(rhs, ","))-1])
		}
		if wall := canonicalWallpaperCandidate(rhs); wall != "" {
			return wall
		}
	}
	return ""
}

func detectFehWallpaper(home string) string {
	path := filepath.Join(home, ".fehbg")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(raw))
	if line == "" {
		return ""
	}
	quotes := []rune{'\'', '"'}
	for _, q := range quotes {
		parts := strings.Split(line, string(q))
		for _, p := range parts {
			if wall := canonicalWallpaperCandidate(p); wall != "" {
				return wall
			}
		}
	}
	return ""
}

func detectSwayWallpaper(home string) string {
	path := filepath.Join(home, ".config", "sway", "config")
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "output ") || !strings.Contains(line, " bg ") {
			continue
		}
		parts := strings.Fields(line)
		for i := 0; i < len(parts); i++ {
			if parts[i] == "bg" && i+1 < len(parts) {
				if wall := canonicalWallpaperCandidate(parts[i+1]); wall != "" {
					return wall
				}
			}
		}
	}
	return ""
}

func newestImageFromCommonDirs(home string) string {
	dirs := []string{
		filepath.Join(home, "Pictures", "Wallpapers"),
		filepath.Join(home, "Pictures", "wallpapers"),
		filepath.Join(home, ".local", "share", "backgrounds"),
	}
	type candidate struct {
		path string
		mod  int64
	}
	all := []candidate{}
	for _, dir := range dirs {
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if !(strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".webp") || strings.HasSuffix(name, ".bmp")) {
				continue
			}
			info, err := e.Info()
			if err != nil || info.Size() <= 0 {
				continue
			}
			all = append(all, candidate{path: filepath.Join(dir, e.Name()), mod: info.ModTime().UnixNano()})
		}
	}
	if len(all) == 0 {
		return ""
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod > all[j].mod })
	return all[0].path
}

func newestImageFromPicturesFiltered(home string) string {
	dir := filepath.Join(home, "Pictures")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	type candidate struct {
		path string
		mod  int64
	}
	all := []candidate{}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if !(strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".webp") || strings.HasSuffix(name, ".bmp")) {
			continue
		}
		if strings.Contains(name, "profile") || strings.Contains(name, "avatar") || strings.Contains(name, "face") || strings.Contains(name, "icon") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() < 200*1024 {
			continue
		}
		all = append(all, candidate{path: filepath.Join(dir, e.Name()), mod: info.ModTime().UnixNano()})
	}
	if len(all) == 0 {
		return ""
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod > all[j].mod })
	return all[0].path
}
