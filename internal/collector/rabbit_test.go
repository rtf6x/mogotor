package collector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCollectRabbitOverviewQueuesAndListeners(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rabbit/api/overview", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "rootfox" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rabbitmq_version": "3.13.7",
			"erlang_version":   "26.2.5",
			"cluster_name":     "rabbit@ci",
			"node":             "rabbit@ci",
			"object_totals": map[string]any{
				"connections": 2,
				"channels":    3,
				"consumers":   1,
				"queues":      1,
				"exchanges":   7,
			},
			"queue_totals": map[string]any{
				"messages":                5,
				"messages_ready":          4,
				"messages_unacknowledged": 1,
			},
			"listeners": []map[string]any{
				{"node": "rabbit@ci", "protocol": "amqp", "ip_address": "::", "port": 5672},
				{"node": "rabbit@ci", "protocol": "http", "ip_address": "::", "port": 15672},
			},
		})
	})
	mux.HandleFunc("/rabbit/api/queues", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name":                    "advice.events",
				"vhost":                   "/",
				"state":                   "running",
				"messages":                5,
				"messages_ready":          4,
				"messages_unacknowledged": 1,
				"consumers":               1,
				"message_stats": map[string]any{
					"publish_details":     map[string]any{"rate": 1.5},
					"deliver_get_details": map[string]any{"rate": 0.5},
				},
			},
		})
	})
	mux.HandleFunc("/rabbit/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name":     "rabbit@ci",
				"running":  true,
				"mem_used": 159825920,
				"uptime":   1101651,
				"type":     "disc",
			},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	snap := CollectRabbit(srv.URL+"/rabbit", "rootfox", "secret")
	if !snap.Available {
		t.Fatalf("expected available, error=%q", snap.Error)
	}
	if snap.Version != "3.13.7" {
		t.Fatalf("version: got %q", snap.Version)
	}
	if snap.ClusterName != "rabbit@ci" || snap.Node != "rabbit@ci" {
		t.Fatalf("cluster/node: %+v", snap)
	}
	if snap.Connections != 2 || snap.Channels != 3 || snap.Consumers != 1 {
		t.Fatalf("object totals: %+v", snap)
	}
	if snap.MessagesReady != 4 || snap.MessagesUnacked != 1 || snap.MessagesTotal != 5 {
		t.Fatalf("queue totals: %+v", snap)
	}
	if len(snap.Listeners) != 2 {
		t.Fatalf("listeners: got %d", len(snap.Listeners))
	}
	if snap.Listeners[0].Protocol != "amqp" || snap.Listeners[0].Port != 5672 {
		t.Fatalf("listener[0]: %+v", snap.Listeners[0])
	}
	if snap.NodeInfo == nil || !snap.NodeInfo.Running || snap.NodeInfo.MemUsedBytes != 159825920 {
		t.Fatalf("node info: %+v", snap.NodeInfo)
	}
	if len(snap.Queues) != 1 {
		t.Fatalf("queues: got %d", len(snap.Queues))
	}
	q := snap.Queues[0]
	if q.Name != "advice.events" || q.Vhost != "/" || q.Consumers != 1 {
		t.Fatalf("queue: %+v", q)
	}
	if q.PublishRate != 1.5 || q.DeliverRate != 0.5 {
		t.Fatalf("queue rates: %+v", q)
	}
}

func TestCollectRabbitEmptyQueues(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/rabbit/api/overview", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rabbitmq_version": "3.13.7",
			"cluster_name":     "rabbit@ci",
			"node":             "rabbit@ci",
			"object_totals": map[string]any{
				"connections": 0,
				"channels":    0,
				"consumers":   0,
				"queues":      0,
				"exchanges":   7,
			},
			"queue_totals": map[string]any{},
			"listeners": []map[string]any{
				{"node": "rabbit@ci", "protocol": "amqp", "ip_address": "127.0.0.1", "port": 5672},
			},
		})
	})
	mux.HandleFunc("/rabbit/api/queues", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]any{})
	})
	mux.HandleFunc("/rabbit/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"name": "rabbit@ci", "running": true, "mem_used": 1, "uptime": 1},
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	snap := CollectRabbit(srv.URL+"/rabbit", "rootfox", "secret")
	if !snap.Available {
		t.Fatalf("expected available, error=%q", snap.Error)
	}
	if len(snap.Queues) != 0 {
		t.Fatalf("expected no queues, got %d", len(snap.Queues))
	}
	if snap.MessagesTotal != 0 || snap.MessagesReady != 0 {
		t.Fatalf("empty queue_totals should be zero: %+v", snap)
	}
	if len(snap.Listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(snap.Listeners))
	}
}

func TestCollectRabbitUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"not_authorised"}`))
	}))
	t.Cleanup(srv.Close)

	snap := CollectRabbit(srv.URL+"/rabbit", "rootfox", "wrong")
	if snap.Available {
		t.Fatal("expected unavailable")
	}
	if snap.Error == "" {
		t.Fatal("expected error")
	}
	if !strings.Contains(strings.ToLower(snap.Error), "401") && !strings.Contains(strings.ToLower(snap.Error), "unauthor") {
		t.Fatalf("expected auth error, got %q", snap.Error)
	}
}

func TestCollectRabbitUnavailable(t *testing.T) {
	snap := CollectRabbit("http://127.0.0.1:1/rabbit", "rootfox", "secret")
	if snap.Available {
		t.Fatal("expected unavailable")
	}
	if snap.Error == "" {
		t.Fatal("expected error")
	}
}
