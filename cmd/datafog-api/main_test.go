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
}
