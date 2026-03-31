package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	internaltheme "github.com/PickleHik3/tooie/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
)

const extractSwatchCacheVersion = "2026-03-29-v1"

type extractSwatchesMsg struct {
	Key      string
	Swatches map[string]string
	err      error
}

type extractSwatchCache struct {
	Version string                        `json:"version"`
	Entries map[string]extractSwatchEntry `json:"entries"`
}

type extractSwatchEntry struct {
	UpdatedAt string            `json:"updated_at"`
	Swatches  map[string]string `json:"swatches"`
}

func loadExtractSwatchesCmd(mode, paletteType string) tea.Cmd {
	return func() tea.Msg {
		key, err := currentExtractSwatchKey(mode, paletteType)
		if err != nil {
			return extractSwatchesMsg{err: err}
		}
		if sw, ok := readExtractSwatchesFromCache(key); ok && len(sw) > 0 {
			return extractSwatchesMsg{Key: key, Swatches: sw}
		}
		sw, err := computeExtractSwatches(mode, paletteType)
		if err != nil {
			return extractSwatchesMsg{Key: key, err: err}
		}
		_ = writeExtractSwatchesToCache(key, sw)
		return extractSwatchesMsg{Key: key, Swatches: sw}
	}
}

func currentExtractSwatchKey(mode, paletteType string) (string, error) {
	wallpaper, err := resolveWallpaperPath()
	if err != nil {
		return "", err
	}
	st, err := os.Stat(wallpaper)
	if err != nil {
		return "", err
	}
	scheme := normalizeSchemeType(paletteType)
	if scheme == "" {
		scheme = normalizeSchemeType(defaultPaletteType)
	}
	resolvedMode := resolveSwatchMode(mode, wallpaper)
	payload := fmt.Sprintf("%s|%d|%d|%s|%s", wallpaper, st.ModTime().UnixNano(), st.Size(), resolvedMode, scheme)
	sum := sha1.Sum([]byte(payload))
	return hex.EncodeToString(sum[:]), nil
}

func resolveSwatchMode(mode, wallpaper string) string {
	m := canonicalMode(mode)
	if m == "dark" || m == "light" {
		return m
	}
	metrics := analyzeWallpaperLuma(wallpaper)
	if brightDominantScene(metrics) && !darkDominantScene(metrics) {
		return "light"
	}
	return "dark"
}

func computeExtractSwatches(mode, paletteType string) (map[string]string, error) {
	wallpaper, err := resolveWallpaperPath()
	if err != nil {
		return nil, err
	}
	matugenBin, err := resolveMatugen("")
	if err != nil {
		return nil, err
	}
	scheme := normalizeSchemeType(paletteType)
	if scheme == "" {
		scheme = normalizeSchemeType(defaultPaletteType)
	}
	fallbackScheme := normalizeSchemeType(defaultPaletteType)
	resolvedMode := resolveSwatchMode(mode, wallpaper)

	out := map[string]string{}
	var lastErr error
	for _, p := range profilePresets {
		profile := canonicalProfile(p)
		idx, prefer := extractionPresetSelection(profile)
		fallback := ""
		if prefer == "closest-to-fallback" {
			fallback = "#6750A4"
		}
		raw, err := runMatugenImage(matugenBin, wallpaper, resolvedMode, scheme, idx, prefer, fallback, "")
		if err != nil && scheme != fallbackScheme {
			raw, err = runMatugenImage(matugenBin, wallpaper, resolvedMode, fallbackScheme, idx, prefer, fallback, "")
		}
		if err != nil {
			lastErr = err
			continue
		}
		roles, err := internaltheme.ParseMatugenColors(raw)
		if err != nil {
			lastErr = err
			continue
		}
		for _, key := range []string{"primary", "secondary", "tertiary", "error"} {
			if c := strings.TrimSpace(roles[key]); internaltheme.IsHexColor(c) {
				out[profile] = internaltheme.NormalizeHex(c)
				break
			}
		}
	}
	if len(out) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("unable to compute extract swatches")
	}
	return out, nil
}

func extractSwatchesCachePath() string {
	return filepath.Join(tooieConfigDir, "cache", "extract-swatches.json")
}

func readExtractSwatchesFromCache(key string) (map[string]string, bool) {
	raw, err := os.ReadFile(extractSwatchesCachePath())
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	var cache extractSwatchCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return nil, false
	}
	if cache.Version != extractSwatchCacheVersion {
		return nil, false
	}
	entry, ok := cache.Entries[key]
	if !ok {
		return nil, false
	}
	if len(entry.Swatches) == 0 {
		return nil, false
	}
	out := map[string]string{}
	for k, v := range entry.Swatches {
		k = canonicalProfile(k)
		v = strings.TrimSpace(v)
		if k == "" || !internaltheme.IsHexColor(v) {
			continue
		}
		out[k] = normalizeHexColor(v)
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func writeExtractSwatchesToCache(key string, swatches map[string]string) error {
	cache := extractSwatchCache{
		Version: extractSwatchCacheVersion,
		Entries: map[string]extractSwatchEntry{},
	}
	if raw, err := os.ReadFile(extractSwatchesCachePath()); err == nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, &cache)
		if cache.Entries == nil {
			cache.Entries = map[string]extractSwatchEntry{}
		}
		cache.Version = extractSwatchCacheVersion
	}
	clean := map[string]string{}
	for k, v := range swatches {
		k = canonicalProfile(k)
		v = strings.TrimSpace(v)
		if k == "" || !internaltheme.IsHexColor(v) {
			continue
		}
		clean[k] = normalizeHexColor(v)
	}
	if len(clean) == 0 {
		return nil
	}
	cache.Entries[key] = extractSwatchEntry{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Swatches:  clean,
	}
	trimExtractSwatchEntries(cache.Entries, 24)
	if err := os.MkdirAll(filepath.Dir(extractSwatchesCachePath()), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(extractSwatchesCachePath(), raw, 0o644)
}

func trimExtractSwatchEntries(entries map[string]extractSwatchEntry, maxEntries int) {
	if len(entries) <= maxEntries {
		return
	}
	type item struct {
		key string
		at  time.Time
	}
	list := make([]item, 0, len(entries))
	for k, v := range entries {
		t, _ := time.Parse(time.RFC3339, strings.TrimSpace(v.UpdatedAt))
		list = append(list, item{key: k, at: t})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].at.After(list[j].at)
	})
	for i := maxEntries; i < len(list); i++ {
		delete(entries, list[i].key)
	}
}
