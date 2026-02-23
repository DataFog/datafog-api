package main

import (
	"testing"
	"time"
)

func TestGetenvDuration(t *testing.T) {
	t.Run("fallback_when_missing", func(t *testing.T) {
		t.Setenv("DATAFOG_READ_TIMEOUT", "")
		got := getenvDuration("DATAFOG_READ_TIMEOUT", 5*time.Second)
		if got != 5*time.Second {
			t.Fatalf("expected fallback duration, got %s", got)
		}
	})

	t.Run("parse_valid_duration", func(t *testing.T) {
		t.Setenv("DATAFOG_READ_TIMEOUT", "7s")
		got := getenvDuration("DATAFOG_READ_TIMEOUT", 5*time.Second)
		if got != 7*time.Second {
			t.Fatalf("expected 7s, got %s", got)
		}
	})

	t.Run("fallback_on_invalid_duration", func(t *testing.T) {
		t.Setenv("DATAFOG_READ_TIMEOUT", "bad")
		got := getenvDuration("DATAFOG_READ_TIMEOUT", 5*time.Second)
		if got != 5*time.Second {
			t.Fatalf("expected fallback duration, got %s", got)
		}
	})

	t.Run("fallback_on_negative_duration", func(t *testing.T) {
		t.Setenv("DATAFOG_READ_TIMEOUT", "-1s")
		got := getenvDuration("DATAFOG_READ_TIMEOUT", 5*time.Second)
		if got != 5*time.Second {
			t.Fatalf("expected fallback duration, got %s", got)
		}
	})

	t.Run("parse_subsecond", func(t *testing.T) {
		t.Setenv("DATAFOG_READ_TIMEOUT", "250ms")
		got := getenvDuration("DATAFOG_READ_TIMEOUT", 5*time.Second)
		if got != 250*time.Millisecond {
			t.Fatalf("expected 250ms, got %s", got)
		}
	})

	t.Run("parse_shutdown_timeout", func(t *testing.T) {
		t.Setenv("DATAFOG_SHUTDOWN_TIMEOUT", "2s")
		got := getenvDuration("DATAFOG_SHUTDOWN_TIMEOUT", 10*time.Second)
		if got != 2*time.Second {
			t.Fatalf("expected 2s, got %s", got)
		}
	})
}

func TestGetenvInt(t *testing.T) {
	t.Run("fallback_when_missing", func(t *testing.T) {
		t.Setenv("DATAFOG_RATE_LIMIT_RPS", "")
		got := getenvInt("DATAFOG_RATE_LIMIT_RPS", 0)
		if got != 0 {
			t.Fatalf("expected fallback int, got %d", got)
		}
	})

	t.Run("parse_valid_int", func(t *testing.T) {
		t.Setenv("DATAFOG_RATE_LIMIT_RPS", "15")
		got := getenvInt("DATAFOG_RATE_LIMIT_RPS", 0)
		if got != 15 {
			t.Fatalf("expected 15, got %d", got)
		}
	})

	t.Run("fallback_on_invalid_int", func(t *testing.T) {
		t.Setenv("DATAFOG_RATE_LIMIT_RPS", "bad")
		got := getenvInt("DATAFOG_RATE_LIMIT_RPS", 3)
		if got != 3 {
			t.Fatalf("expected fallback int, got %d", got)
		}
	})

	t.Run("fallback_on_negative_int", func(t *testing.T) {
		t.Setenv("DATAFOG_RATE_LIMIT_RPS", "-1")
		got := getenvInt("DATAFOG_RATE_LIMIT_RPS", 3)
		if got != 3 {
			t.Fatalf("expected fallback int, got %d", got)
		}
	})

	t.Run("parse_zero", func(t *testing.T) {
		t.Setenv("DATAFOG_RATE_LIMIT_RPS", "0")
		got := getenvInt("DATAFOG_RATE_LIMIT_RPS", 3)
		if got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})
}
