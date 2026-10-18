package exec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leo-ramos/als/internal/config"
	"github.com/leo-ramos/als/internal/store"
)

func setupTestStore(t *testing.T) (*store.Store, func()) {
	tmpDir, err := os.MkdirTemp("", "als-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		ConfigDir: filepath.Join(tmpDir, "config"),
		BinDir:    filepath.Join(tmpDir, "bin"),
	}
	st := store.New(cfg)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	return st, cleanup
}

func TestResolveVariables_PositionalDirect(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	cmdDef := &store.Command{
		Cmd: "fvm flutter build appbundle --dart-define URL={{url}}",
		Vars: map[string]store.VarDef{
			"url": {
				Positional: true,
				Prompt:     "Server URL",
			},
		},
	}
	compiled, err := CompileTemplate(cmdDef.Cmd, cmdDef.Vars)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// 1. Direct positional parameter
	args := []string{"https://tabletserveroci.casadoadubo.com.br:8601"}
	res, err := ResolveVariables("buildCdaForcaAdr", cmdDef, compiled, args, st)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}

	if res.ResolvedVars["url"] != "https://tabletserveroci.casadoadubo.com.br:8601" {
		t.Errorf("url = %q, want https://tabletserveroci.casadoadubo.com.br:8601", res.ResolvedVars["url"])
	}
	if len(res.RemainingArgs) != 0 {
		t.Errorf("expected no remaining args, got %v", res.RemainingArgs)
	}

	// 2. Direct positional parameter with trailing flag for the underlying command
	argsWithFlag := []string{"https://tabletserveroci.casadoadubo.com.br:8601", "--verbose"}
	res2, err := ResolveVariables("buildCdaForcaAdr", cmdDef, compiled, argsWithFlag, st)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if res2.ResolvedVars["url"] != "https://tabletserveroci.casadoadubo.com.br:8601" {
		t.Errorf("url = %q, want https://tabletserveroci.casadoadubo.com.br:8601", res2.ResolvedVars["url"])
	}
	if len(res2.RemainingArgs) != 1 || res2.RemainingArgs[0] != "--verbose" {
		t.Errorf("expected remaining args [--verbose], got %v", res2.RemainingArgs)
	}

	// 3. Flags before or starting with - should not be consumed as parameter
	argsOnlyFlag := []string{"-v"}
	// In non-interactive mode without stored value, this should fail since url is missing
	_, err = ResolveVariables("buildCdaForcaAdr", cmdDef, compiled, argsOnlyFlag, st)
	if err == nil {
		t.Errorf("expected error for missing variable when only flags provided in non-interactive mode")
	}
}

func TestResolveVariables_PositionalWithDefault(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	cmdDef := &store.Command{
		Cmd: "fvm flutter build appbundle --dart-define URL={{url}}",
		Vars: map[string]store.VarDef{
			"url": {
				Default:    "https://default.url.com",
				Positional: true,
			},
		},
	}
	compiled, _ := CompileTemplate(cmdDef.Cmd, cmdDef.Vars)

	// Call without args: should use default
	res, err := ResolveVariables("buildCdaForcaAdr", cmdDef, compiled, []string{}, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ResolvedVars["url"] != "https://default.url.com" {
		t.Errorf("url = %q, want default https://default.url.com", res.ResolvedVars["url"])
	}

	// Call with override arg: should use arg
	resOverride, err := ResolveVariables("buildCdaForcaAdr", cmdDef, compiled, []string{"https://override.url.com"}, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resOverride.ResolvedVars["url"] != "https://override.url.com" {
		t.Errorf("url = %q, want override https://override.url.com", resOverride.ResolvedVars["url"])
	}
}

func TestResolveVariables_DelimiterProtection(t *testing.T) {
	st, cleanup := setupTestStore(t)
	defer cleanup()

	cmdDef := &store.Command{
		Cmd: "ssh ubuntu@{{server}}",
		Vars: map[string]store.VarDef{
			"server": {
				Default: "10.0.0.1",
			},
		},
	}
	compiled, _ := CompileTemplate(cmdDef.Cmd, cmdDef.Vars)

	// Using -- to pass commands to ssh
	res, err := ResolveVariables("sshCmd", cmdDef, compiled, []string{"--", "ls", "-la"}, st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ResolvedVars["server"] != "10.0.0.1" {
		t.Errorf("server = %q, want 10.0.0.1", res.ResolvedVars["server"])
	}
	if len(res.RemainingArgs) != 2 || res.RemainingArgs[0] != "ls" || res.RemainingArgs[1] != "-la" {
		t.Errorf("expected remaining args [ls, -la], got %v", res.RemainingArgs)
	}
}
