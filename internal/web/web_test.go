package web_test

import "testing"

func TestAppServesPagesAndSurvivesRestart(t *testing.T) {
	ta := newTestApp(t)
	if r := ta.get("/"); r.Status != 200 {
		t.Fatalf("GET / = %d, want 200", r.Status)
	}
	// Restarting on the same database re-runs migrations without error.
	ta.restart()
	if r := ta.get("/"); r.Status != 200 {
		t.Fatalf("GET / after restart = %d, want 200", r.Status)
	}
}
