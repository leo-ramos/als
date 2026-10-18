package linter

import (
	"strings"
	"testing"
)

func TestAnalyze_FlutterDartDefineURL(t *testing.T) {
	cmd := "fvm flutter build appbundle --flavor cda -t lib/main_cda.dart --dart-define URL=https://tabletserveroci.casadoadubo.com.br:8601"
	warnings := Analyze(cmd)

	if len(warnings) == 0 {
		t.Fatalf("expected warnings for URL=https://..., got 0")
	}

	var found bool
	for _, w := range warnings {
		if w.Type == "key_value_url" {
			found = true
			if w.VariableKey != "url" {
				t.Errorf("VariableKey = %q, want 'url'", w.VariableKey)
			}
			if w.TargetSpan != "URL=https://tabletserveroci.casadoadubo.com.br:8601" {
				t.Errorf("TargetSpan = %q, want 'URL=https://tabletserveroci.casadoadubo.com.br:8601'", w.TargetSpan)
			}
			if w.ReplacementSpan != "URL={{url}}" {
				t.Errorf("ReplacementSpan = %q, want 'URL={{url}}'", w.ReplacementSpan)
			}
			if w.RawMatch != "https://tabletserveroci.casadoadubo.com.br:8601" {
				t.Errorf("RawMatch = %q, want 'https://tabletserveroci.casadoadubo.com.br:8601'", w.RawMatch)
			}

			// Test precise span replacement
			replaced := strings.Replace(cmd, w.TargetSpan, w.ReplacementSpan, 1)
			expected := "fvm flutter build appbundle --flavor cda -t lib/main_cda.dart --dart-define URL={{url}}"
			if replaced != expected {
				t.Errorf("replaced string = %q, want %q", replaced, expected)
			}
		}
	}

	if !found {
		t.Errorf("did not find key_value_url warning")
	}
}

func TestAnalyze_StandaloneURL(t *testing.T) {
	cmd := "curl https://api.domain.com/v1/data"
	warnings := Analyze(cmd)

	var found bool
	for _, w := range warnings {
		if w.Type == "raw_url" && w.VariableKey == "url" {
			found = true
			if w.TargetSpan != "https://api.domain.com/v1/data" {
				t.Errorf("TargetSpan = %q", w.TargetSpan)
			}
			if w.ReplacementSpan != "{{url}}" {
				t.Errorf("ReplacementSpan = %q", w.ReplacementSpan)
			}
		}
	}
	if !found {
		t.Errorf("expected raw_url warning")
	}
}

func TestAnalyze_DefineEnv(t *testing.T) {
	cmd := "docker run --env PORT=8080 -d myimage"
	warnings := Analyze(cmd)

	var found bool
	for _, w := range warnings {
		if w.Type == "define_var" && w.VariableKey == "port" {
			found = true
			if w.TargetSpan != "PORT=8080" {
				t.Errorf("TargetSpan = %q", w.TargetSpan)
			}
			if w.ReplacementSpan != "PORT={{port}}" {
				t.Errorf("ReplacementSpan = %q", w.ReplacementSpan)
			}
		}
	}
	if !found {
		t.Errorf("expected define_var warning for PORT=8080")
	}
}
