// Package observe — config_test.go pins the env-var contract for the
// OTLP/Langfuse integration. Regression guard: a future operator who
// renames OTLP_* variables should fail loudly here.
package observe

import (
	"testing"
	"time"
)

// TestLoadConfig_OTLPEnvVars covers the design.md §4.5 contract: the
// Langfuse OTLP endpoint is configured via OTLP_ENDPOINT,
// OTLP_LANGFUSE_PUBLIC_KEY, OTLP_LANGFUSE_SECRET_KEY (Langfuse is OTLP-
// compatible; v3+ accepts a project pair at the OTel ingestion path).
func TestLoadConfig_OTLPEnvVars(t *testing.T) {
	t.Setenv("OTLP_ENDPOINT", "https://langfuse.example.com")
	t.Setenv("OTLP_LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("OTLP_LANGFUSE_SECRET_KEY", "sk-test")
	// Unset the legacy LANGFUSE_* keys so the test isn't accidentally
	// satisfied through the fork's old behaviour.
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")
	t.Setenv("LANGFUSE_ENABLED", "")

	cfg := LoadConfig()

	if !cfg.Enabled {
		t.Fatalf("expected Enabled=true when OTLP_* keys are set, got false")
	}
	if cfg.Endpoint != "https://langfuse.example.com" {
		t.Errorf("Endpoint = %q, want https://langfuse.example.com", cfg.Endpoint)
	}
	if cfg.PublicKey != "pk-test" {
		t.Errorf("PublicKey = %q, want pk-test", cfg.PublicKey)
	}
	if cfg.SecretKey != "sk-test" {
		t.Errorf("SecretKey = %q, want sk-test", cfg.SecretKey)
	}
}

// TestLoadConfig_DisabledWithoutEnvVars ensures no env vars -> Enabled
// is false (safe default). Operators can leave a deployment unconfigured
// and Observability simply falls back to no-op.
func TestLoadConfig_DisabledWithoutEnvVars(t *testing.T) {
	t.Setenv("OTLP_ENDPOINT", "")
	t.Setenv("OTLP_LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("OTLP_LANGFUSE_SECRET_KEY", "")
	t.Setenv("OTLP_ENABLED", "")
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")
	t.Setenv("LANGFUSE_ENABLED", "")

	cfg := LoadConfig()
	if cfg.Enabled {
		t.Errorf("expected Enabled=false when no env vars set, got true")
	}
}

// TestLoadConfig_DisabledOnPartialKeys: a half-configured OTLP path
// (endpoint only, no credentials) MUST NOT be flagged Enabled. We want
// to fail closed rather than start emitting unauthorised spans (and
// flood the dashboard with sample rows).
func TestLoadConfig_DisabledOnPartialKeys(t *testing.T) {
	t.Setenv("OTLP_ENDPOINT", "https://langfuse.example.com")
	t.Setenv("OTLP_LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("OTLP_LANGFUSE_SECRET_KEY", "")
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")
	t.Setenv("LANGFUSE_ENABLED", "")

	cfg := LoadConfig()
	if cfg.Enabled {
		t.Errorf("Enabled must be false when secret key is missing, got true")
	}
}

// TestLoadConfig_DefaultsAndParse covers the helper knobs that an
// operator might want to tune (flush interval, sample rate, request
// timeout). The defaults are conservative to keep memory + network cost
// bounded even when the page of spans grows large.
func TestLoadConfig_DefaultsAndParse(t *testing.T) {
	t.Setenv("OTLP_ENDPOINT", "https://langfuse.example.com")
	t.Setenv("OTLP_LANGFUSE_PUBLIC_KEY", "pk-test")
	t.Setenv("OTLP_LANGFUSE_SECRET_KEY", "sk-test")
	t.Setenv("LANGFUSE_HOST", "")
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")
	t.Setenv("LANGFUSE_ENABLED", "")
	t.Setenv("OTLP_FLUSH_INTERVAL", "500ms")
	t.Setenv("OTLP_REQUEST_TIMEOUT", "5s")
	t.Setenv("OTLP_SAMPLE_RATE", "0.25")

	cfg := LoadConfig()

	if cfg.FlushInterval != 500*time.Millisecond {
		t.Errorf("FlushInterval = %v, want 500ms", cfg.FlushInterval)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Errorf("RequestTimeout = %v, want 5s", cfg.RequestTimeout)
	}
	if cfg.SampleRate != 0.25 {
		t.Errorf("SampleRate = %v, want 0.25", cfg.SampleRate)
	}
}

// TestConfigValidate covers the validation surface. Disabled passes
// trivially; enabled-without-host and enabled-without-keys fail.
func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"disabled is always valid", Config{Enabled: false}, false},
		{"enabled without host fails", Config{Enabled: true, PublicKey: "pk", SecretKey: "sk"}, true},
		{"enabled without keys fails", Config{Enabled: true, Endpoint: "https://x"}, true},
		{"enabled with all fields passes", Config{Enabled: true, Endpoint: "https://x", PublicKey: "pk", SecretKey: "sk"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestSpanKindLabels pins the canonical strings for span categorisation
// (matches the categories listed in prd.md §3.1 AC-B8). The labels feed
// the Langfuse UI filter dropdowns and must NOT drift without notice.
func TestSpanKindLabels(t *testing.T) {
	cases := map[SpanKind]string{
		KindDB:     "db",
		KindES:     "es",
		KindSAG:    "sag",
		KindLLM:    "llm",
		KindHTTP:   "http",
		KindAsynq:  "asynq",
	}
	for got, want := range cases {
		if string(got) != want {
			t.Errorf("SpanKind %q drift: want %q", got, want)
		}
	}
}
