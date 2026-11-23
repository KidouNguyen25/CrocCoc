package main

import "testing"

func TestResolveWorkerPathPrefersEnvironmentOverride(t *testing.T) {
	t.Setenv("CROCCOC_WORKER_PATH", `  C:\dev\worker.exe  `)
	if got := resolveWorkerPath(); got != `C:\dev\worker.exe` {
		t.Fatalf("resolveWorkerPath() = %q, want trimmed override", got)
	}
}

func TestResolveWorkerPathWithoutBundledWorkerIsEmpty(t *testing.T) {
	t.Setenv("CROCCOC_WORKER_PATH", "")
	// Binary kiểm thử không có bin/croccoc-engine.exe bên cạnh nên phải trả về rỗng.
	if got := resolveWorkerPath(); got != "" {
		t.Fatalf("resolveWorkerPath() = %q, want empty when no bundled worker exists", got)
	}
}
