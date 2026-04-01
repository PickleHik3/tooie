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

func TestBestWallpaperPathFallsBackToPictures(t *testing.T) {
	tmp := t.TempDir()
	pics := filepath.Join(tmp, "Pictures", "Wallpapers")
	if err := os.MkdirAll(pics, 0o755); err != nil {
		t.Fatal(err)
	}
	img := filepath.Join(pics, "wallpaper.jpg")
	if err := os.WriteFile(img, []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOOIE_WALLPAPER", "")
	got, ok := bestWallpaperPath(tmp)
	if !ok {
		t.Fatalf("expected wallpapers fallback to resolve")
	}
	if got != img {
		t.Fatalf("bestWallpaperPath() = %q, want %q", got, img)
	}
}

func TestBestWallpaperPathFilteredPicturesSkipsProfileNames(t *testing.T) {
	tmp := t.TempDir()
	pics := filepath.Join(tmp, "Pictures")
	if err := os.MkdirAll(pics, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(pics, "profile.png")
	if err := os.WriteFile(profile, make([]byte, 300*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	wall := filepath.Join(pics, "mountains_wallpaper.jpg")
	if err := os.WriteFile(wall, make([]byte, 350*1024), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := bestWallpaperPath(tmp)
	if !ok {
		t.Fatalf("expected fallback wallpaper to resolve")
	}
	if got != wall {
		t.Fatalf("bestWallpaperPath() = %q, want %q", got, wall)
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
