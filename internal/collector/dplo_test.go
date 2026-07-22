package collector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCountDploProjects(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "projects")
	for _, spec := range []struct {
		id      string
		enabled bool
	}{
		{"alpha", true},
		{"beta", false},
		{"gamma", true},
	} {
		dir := filepath.Join(projects, spec.id)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{"enabled": spec.enabled})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "project.json"), data, 0o640); err != nil {
			t.Fatal(err)
		}
	}

	total, enabled := countDploProjects(root)
	if total != 3 {
		t.Fatalf("expected 3 projects, got %d", total)
	}
	if enabled != 2 {
		t.Fatalf("expected 2 enabled projects, got %d", enabled)
	}
}

func TestCountDploRuns(t *testing.T) {
	root := t.TempDir()
	runs := filepath.Join(root, "runs")
	for _, spec := range []struct {
		id     string
		status string
	}{
		{"run-1", "success"},
		{"run-2", "running"},
		{"run-3", "queued"},
	} {
		dir := filepath.Join(runs, spec.id)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{
			"projectId": "demo",
			"status":    spec.status,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o640); err != nil {
			t.Fatal(err)
		}
	}

	total, running := countDploRuns(root)
	if total != 3 {
		t.Fatalf("expected 3 runs, got %d", total)
	}
	if running != 2 {
		t.Fatalf("expected 2 active runs, got %d", running)
	}
}

func TestActiveDploRun(t *testing.T) {
	root := t.TempDir()
	runs := filepath.Join(root, "runs")
	for _, spec := range []struct {
		id        string
		projectID string
		status    string
	}{
		{"run-old", "old", "success"},
		{"run-live", "live", "running"},
	} {
		dir := filepath.Join(runs, spec.id)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(map[string]any{
			"projectId": spec.projectID,
			"status":    spec.status,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o640); err != nil {
			t.Fatal(err)
		}
	}

	projectID, runID := activeDploRun(root)
	if projectID != "live" || runID != "run-live" {
		t.Fatalf("expected live/run-live, got %s/%s", projectID, runID)
	}
}
