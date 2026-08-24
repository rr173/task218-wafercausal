package ingest

import (
	"testing"
)

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint("WFR-1", "inspect-1", 10, 10, 2, "2026-08-23T08:30:00Z")
	b := Fingerprint("WFR-1", "inspect-1", 10, 10, 2, "2026-08-23T08:30:00Z")
	if a != b {
		t.Fatalf("fingerprint not stable: %s != %s", a, b)
	}
	c := Fingerprint("WFR-1", "inspect-1", 10.1, 10, 2, "2026-08-23T08:30:00Z")
	if a == c {
		t.Fatalf("fingerprint should differ for different coordinate")
	}
}

func TestNormalizeSeverity(t *testing.T) {
	cases := map[string]string{
		"major":     "major",
		"MAJOR":     "major",
		"critical":  "critical",
		"":          "minor",
		"unknown":   "minor",
	}
	for in, want := range cases {
		if got := normalizeSeverity(in); got != want {
			t.Fatalf("normalizeSeverity(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeParam(t *testing.T) {
	if got := normalizeParam(""); got != "{}" {
		t.Fatalf("empty param should be {}, got %s", got)
	}
	if got := normalizeParam(`{"a":1}`); got != `{"a":1}` {
		t.Fatalf("param should pass through, got %s", got)
	}
}
