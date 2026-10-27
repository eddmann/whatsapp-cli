package cli

import (
	"testing"
	"time"
)

func TestParseBackfillAfter(t *testing.T) {
	if got, err := parseBackfillAfter(""); err != nil || !got.IsZero() {
		t.Fatalf("expected zero time for empty value, got %v (%v)", got, err)
	}

	got, err := parseBackfillAfter("2026-04-22")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	if y, m, d := got.Date(); y != 2026 || m != time.April || d != 22 {
		t.Fatalf("unexpected date %v", got)
	}

	got, err = parseBackfillAfter("2026-04-22T10:00:00Z")
	if err != nil || !got.Equal(time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected RFC3339 parse %v (%v)", got, err)
	}

	if _, err := parseBackfillAfter("April 22"); err == nil {
		t.Fatalf("expected error for invalid value")
	}
}
