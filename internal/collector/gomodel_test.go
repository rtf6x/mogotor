package collector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollectGoModelEnabledModelsAvailability(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/models", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"selector":      "opencode-zen/big-pickle",
				"provider_name": "opencode-zen",
				"provider_type": "openai",
				"model":         map[string]any{"id": "big-pickle"},
				"access":        map[string]any{"effective_enabled": true},
			},
			{
				"selector":      "opencode-zen/mimo-v2.5-free",
				"provider_name": "opencode-zen",
				"provider_type": "openai",
				"model":         map[string]any{"id": "mimo-v2.5-free"},
				"access":        map[string]any{"effective_enabled": true},
			},
			{
				"selector":      "google/gemma-4-26b-a4b-it:free",
				"provider_name": "openrouter",
				"provider_type": "openrouter",
				"model":         map[string]any{"id": "google/gemma-4-26b-a4b-it:free"},
				"access":        map[string]any{"effective_enabled": false},
			},
			{
				"selector":      "opencode-zen/laguna-s-2.1-free",
				"provider_name": "opencode-zen",
				"provider_type": "openai",
				"model":         map[string]any{"id": "laguna-s-2.1-free"},
				"access":        map[string]any{"effective_enabled": true},
			},
		})
	})
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "opencode-zen/big-pickle"},
				{"id": "big-pickle"},
				{"id": "opencode-zen/mimo-v2.5-free"},
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	snap := CollectGoModel(srv.URL, "test-key")
	if !snap.Available {
		t.Fatalf("expected available, error=%q", snap.Error)
	}
	if len(snap.Models) != 3 {
		t.Fatalf("expected 3 enabled models, got %d: %+v", len(snap.Models), snap.Models)
	}

	byID := map[string]bool{}
	for _, m := range snap.Models {
		byID[m.Selector] = m.Available
	}
	if !byID["opencode-zen/big-pickle"] {
		t.Fatalf("big-pickle should be available: %+v", snap.Models)
	}
	if !byID["opencode-zen/mimo-v2.5-free"] {
		t.Fatalf("mimo should be available: %+v", snap.Models)
	}
	if byID["opencode-zen/laguna-s-2.1-free"] {
		t.Fatalf("laguna should be unavailable (missing from /v1/models): %+v", snap.Models)
	}
	if _, ok := byID["google/gemma-4-26b-a4b-it:free"]; ok {
		t.Fatalf("disabled gemma must not appear: %+v", snap.Models)
	}
}

func TestCollectGoModelEmptyURL(t *testing.T) {
	snap := CollectGoModel("", "key")
	if snap.Available {
		t.Fatal("expected unavailable")
	}
	if snap.Error == "" {
		t.Fatal("expected error")
	}
	if snap.Models == nil {
		t.Fatal("models slice must be non-nil")
	}
}

func TestCollectGoModelUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	t.Cleanup(srv.Close)

	snap := CollectGoModel(srv.URL, "bad")
	if snap.Available {
		t.Fatal("expected unavailable")
	}
	if snap.Error == "" {
		t.Fatal("expected error")
	}
}
