package main

import (
	"os"
	"strings"
	"testing"
)

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

func TestResolveRelayConfig(t *testing.T) {
	tests := []struct {
		name        string
		addressSet  bool
		passwordSet bool
		address     string
		password    string
		wantAddress string
		wantPass    string
		wantLabel   string
		wantErr     bool
	}{
		{
			name:        "defaults when unset",
			wantAddress: defaultPublicRelayAddress,
			wantPass:    defaultPublicRelayPassword,
			wantLabel:   "Relay công cộng: " + defaultPublicRelayAddress,
		},
		{
			name:        "custom pair",
			addressSet:  true,
			passwordSet: true,
			address:     "relay.example.test:9009",
			password:    "private-password",
			wantAddress: "relay.example.test:9009",
			wantPass:    "private-password",
			wantLabel:   "Relay tùy chỉnh",
		},
		{name: "password without address", passwordSet: true, password: "private-password", wantErr: true},
		{name: "address without password", addressSet: true, address: "relay.example.test:9009", wantErr: true},
		{name: "empty address is not treated as unset", addressSet: true, wantErr: true},
		{name: "empty password is not treated as unset", addressSet: true, passwordSet: true, address: "relay.example.test:9009", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, key := range []string{"CROCCOC_RELAY_ADDRESS", "CROCCOC_RELAY_PASSWORD"} {
				value, wasSet := os.LookupEnv(key)
				t.Cleanup(func() {
					if wasSet {
						_ = os.Setenv(key, value)
					} else {
						_ = os.Unsetenv(key)
					}
				})
				_ = os.Unsetenv(key)
			}
			if test.addressSet {
				if err := os.Setenv("CROCCOC_RELAY_ADDRESS", test.address); err != nil {
					t.Fatal(err)
				}
			}
			if test.passwordSet {
				if err := os.Setenv("CROCCOC_RELAY_PASSWORD", test.password); err != nil {
					t.Fatal(err)
				}
			}

			address, password, label, configErr := resolveRelayConfig()
			if test.wantErr {
				if configErr == nil || !strings.HasPrefix(configErr.Error(), "ENGINE_UNAVAILABLE:") {
					t.Fatalf("resolveRelayConfig() error = %v, want ENGINE_UNAVAILABLE", configErr)
				}
				return
			}
			if configErr != nil {
				t.Fatalf("resolveRelayConfig() error = %v", configErr)
			}
			if address != test.wantAddress || password != test.wantPass || label != test.wantLabel {
				t.Fatalf("resolveRelayConfig() = (%q, %q, %q), want (%q, %q, %q)",
					address, password, label, test.wantAddress, test.wantPass, test.wantLabel)
			}
		})
	}
}

func TestEngineServiceConfigUnsetsRelayPasswordAfterReading(t *testing.T) {
	t.Setenv("CROCCOC_RELAY_ADDRESS", "relay.example.test:9009")
	t.Setenv("CROCCOC_RELAY_PASSWORD", "private-password")
	t.Setenv("CROCCOC_WORKER_PATH", "")

	config := engineServiceConfig(nil)
	if config.RelayPassword != "private-password" {
		t.Fatalf("engineServiceConfig() relay password = %q, want configured password", config.RelayPassword)
	}
	if got := os.Getenv("CROCCOC_RELAY_PASSWORD"); got != "" {
		t.Fatalf("CROCCOC_RELAY_PASSWORD remains in host environment: %q", got)
	}
}
