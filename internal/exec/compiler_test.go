package exec

import (
	"testing"
)

func TestCompileTemplate(t *testing.T) {
	tests := []struct {
		name         string
		cmd          string
		wantShellCmd string
		wantVars     []string
	}{
		{
			name:         "ssh with double quotes around key and unquoted ip",
			cmd:          `ssh -i "{{key}}" ubuntu@{{server_ip}}`,
			wantShellCmd: `ssh -i "$ALS_VAR_KEY" ubuntu@"$ALS_VAR_SERVER_IP"`,
			wantVars:     []string{"key", "server_ip"},
		},
		{
			name:         "prefix unquoted URL={{url}}",
			cmd:          `fvm flutter build appbundle --flavor cda -t lib/main_cda.dart --dart-define URL={{url}}`,
			wantShellCmd: `fvm flutter build appbundle --flavor cda -t lib/main_cda.dart --dart-define URL="$ALS_VAR_URL"`,
			wantVars:     []string{"url"},
		},
		{
			name:         "outer double quotes with prefix",
			cmd:          `fvm flutter build appbundle --dart-define "URL={{url}}"`,
			wantShellCmd: `fvm flutter build appbundle --dart-define "URL=$ALS_VAR_URL"`,
			wantVars:     []string{"url"},
		},
		{
			name:         "outer single quotes with prefix",
			cmd:          `fvm flutter build appbundle --dart-define 'URL={{url}}'`,
			wantShellCmd: `fvm flutter build appbundle --dart-define 'URL='"$ALS_VAR_URL"''`,
			wantVars:     []string{"url"},
		},
		{
			name:         "bearer token in quotes",
			cmd:          `curl -H "Authorization: Bearer {{token}}" https://api.com`,
			wantShellCmd: `curl -H "Authorization: Bearer $ALS_VAR_TOKEN" https://api.com`,
			wantVars:     []string{"token"},
		},
		{
			name:         "single quotes directly around placeholder",
			cmd:          `echo '{{message}}'`,
			wantShellCmd: `echo "$ALS_VAR_MESSAGE"`,
			wantVars:     []string{"message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileTemplate(tt.cmd, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ShellCmd != tt.wantShellCmd {
				t.Errorf("CompileTemplate() ShellCmd = %q, want %q", got.ShellCmd, tt.wantShellCmd)
			}
			if len(got.Variables) != len(tt.wantVars) {
				t.Errorf("CompileTemplate() Variables = %v, want %v", got.Variables, tt.wantVars)
			} else {
				for i, v := range got.Variables {
					if v != tt.wantVars[i] {
						t.Errorf("CompileTemplate() Variables[%d] = %q, want %q", i, v, tt.wantVars[i])
					}
				}
			}
		})
	}
}
