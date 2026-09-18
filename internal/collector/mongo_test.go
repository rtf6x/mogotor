package collector

import "testing"

func TestMongoStatusRawToModel(t *testing.T) {
	raw := mongoStatusRaw{
		Version:              "4.4.29",
		Uptime:               3600,
		Connections:          12,
		ConnectionsAvailable: 51180,
		MemResident:          85,
		MemVirtual:           1593,
		CacheBytes:           10286371,
		CacheMaxBytes:        1481637888,
		OpsQuery:             230436,
		OpsInsert:            435,
		OpsUpdate:            118220,
		OpsDelete:            204,
	}

	model := raw.toModel()
	if model.Version != "4.4.29" {
		t.Fatalf("unexpected version: %s", model.Version)
	}
	if model.MemoryResidentMb != 85 {
		t.Fatalf("unexpected resident memory: %d", model.MemoryResidentMb)
	}
	if model.CacheBytes != 10286371 {
		t.Fatalf("unexpected cache bytes: %d", model.CacheBytes)
	}
}

func TestBytesTrim(t *testing.T) {
	if got := string(bytesTrim([]byte("  {\"ok\":true}\n"))); got != `{"ok":true}` {
		t.Fatalf("unexpected trim: %q", got)
	}
}

func TestContainerMongoURIRewritesLoopbackPort(t *testing.T) {
	got, ok := containerMongoURI("mongodb://rootfox:secret@127.0.0.1:28888/admin")
	if !ok {
		t.Fatalf("expected loopback uri to rewrite")
	}
	if got != "mongodb://rootfox:secret@127.0.0.1:27017/admin" {
		t.Fatalf("unexpected rewritten uri: %q", got)
	}
}

func TestContainerMongoURIRejectsNonLoopbackHost(t *testing.T) {
	if _, ok := containerMongoURI("mongodb://db.example.com:28888/admin"); ok {
		t.Fatalf("expected non-loopback host to be rejected")
	}
}
