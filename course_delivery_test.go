package main

import (
	"testing"
	"time"
)

func TestDeadlineDecision(t *testing.T) {
	now := time.Date(2026, 9, 20, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		deadline time.Time
		wantOK   bool
	}{
		{"future lesson", now.Add(time.Hour), true},
		{"deadline reached", now, false},
		{"late upload", now.Add(-time.Minute), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decide(publishRequest{"math-101", "unit-3", "aGVsbG8=", tc.deadline}, now)
			if (err == nil) != tc.wantOK {
				t.Fatalf("decision: %v", err)
			}
			if tc.wantOK && (got.ObjectKey != "math-101/unit-3.webp" || got.Status != "published") {
				t.Fatalf("delivery: %+v", got)
			}
		})
	}
}
