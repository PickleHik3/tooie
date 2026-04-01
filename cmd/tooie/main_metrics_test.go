package main

import "testing"

func TestEffectiveMemoryUsedBytesPrefersTotalMinusAvailable(t *testing.T) {
	got := effectiveMemoryUsedBytes(8_000, 5_000, 1_000)
	if got != 3_000 {
		t.Fatalf("effectiveMemoryUsedBytes = %d, want %d", got, 3_000)
	}
}

func TestEffectiveMemoryUsedBytesFallsBackToUsed(t *testing.T) {
	got := effectiveMemoryUsedBytes(8_000, 0, 1_250)
	if got != 1_250 {
		t.Fatalf("effectiveMemoryUsedBytes = %d, want %d", got, 1_250)
	}
}
