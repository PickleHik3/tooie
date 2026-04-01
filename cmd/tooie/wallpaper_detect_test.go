package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBestWallpaperPathUsesEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	img := filepath.Join(tmp, "wall.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOOIE_WALLPAPER", img)
	got, ok := bestWallpaperPath(tmp)
	if !ok {
		t.Fatalf("expected env wallpaper to resolve")
	}
	if got != img {
		t.Fatalf("bestWallpaperPath() = %q, want %q", got, img)
	}
}

func TestBestWallpaperPathFallsBackToRememberedCache(t *testing.T) {
	tmp := t.TempDir()
	img := filepath.Join(tmp, "wall.png")
	if err := os.WriteFile(img, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	rememberWallpaperPath(tmp, img)
	t.Setenv("TOOIE_WALLPAPER", "")
	got, ok := bestWallpaperPath(tmp)
	if !ok {
		t.Fatalf("expected cached wallpaper fallback to resolve")
	}
	if got != img {
		t.Fatalf("bestWallpaperPath() = %q, want %q", got, img)
	}
}

func TestBestWallpaperPathFallsBackToTermuxBackground(t *testing.T) {
	tmp := t.TempDir()
	img := filepath.Join(tmp, ".termux", "background", "background.jpeg")
	if err := os.MkdirAll(filepath.Dir(img), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(img, []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOOIE_WALLPAPER", "")
	got, ok := bestWallpaperPath(tmp)
	if !ok {
		t.Fatalf("expected termux background fallback to resolve")
	}
	if got != img {
		t.Fatalf("bestWallpaperPath() = %q, want %q", got, img)
	}
}
