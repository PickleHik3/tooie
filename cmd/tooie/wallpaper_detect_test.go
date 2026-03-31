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
	pics := filepath.Join(tmp, "Pictures")
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
		t.Fatalf("expected pictures fallback wallpaper to resolve")
	}
	if got != img {
		t.Fatalf("bestWallpaperPath() = %q, want %q", got, img)
	}
}
