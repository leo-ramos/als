package linter

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// Regex matching potential IPv4 addresses
	ipPattern = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	// Regex matching common flags followed by file paths: -i, -f, --file, --key, --cert, etc.
	fileFlagPattern = regexp.MustCompile(`(-[a-zA-Z]|--[a-zA-Z0-9_-]+)[= ]+["']?([^"'\s]+)["']?`)
	// Regex matching potential tokens / secret assignments
	tokenPattern = regexp.MustCompile(`(?i)(token|password|secret|bearer|key)[= :]+["']?([a-zA-Z0-9_\-\.]{12,})["']?`)
)

type LintWarning struct {
	Type        string // "relative_path", "raw_ip", "raw_token"
	RawMatch    string
	Suggestion  string
	VariableKey string
}

// Analyze inspects a raw command string and flags security and portability issues.
func Analyze(rawCmd string) []LintWarning {
	var warnings []LintWarning

	// 1. Check for raw IPs
	for _, match := range ipPattern.FindAllString(rawCmd, -1) {
		if ip := net.ParseIP(match); ip != nil && !ip.IsLoopback() {
			warnings = append(warnings, LintWarning{
				Type:        "raw_ip",
				RawMatch:    match,
				Suggestion:  fmt.Sprintf("Raw public IP '%s' detected. Consider parameterizing with {{server_ip}} to keep secrets out of Git.", match),
				VariableKey: "server_ip",
			})
		}
	}

	// 2. Check for relative file flags (-i "chave.key", etc.)
	matches := fileFlagPattern.FindAllStringSubmatch(rawCmd, -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		pathVal := m[2]
		// Skip URLs, placeholders, or special options
		if strings.HasPrefix(pathVal, "http") || strings.HasPrefix(pathVal, "{{") || strings.HasPrefix(pathVal, "-") {
			continue
		}
		// If path doesn't start with / or ~ or $HOME
		if !strings.HasPrefix(pathVal, "/") && !strings.HasPrefix(pathVal, "~") && !strings.HasPrefix(pathVal, "$") {
			absPath, err := filepath.Abs(pathVal)
			if err == nil {
				// Check if file exists in current working directory
				if _, statErr := os.Stat(pathVal); statErr == nil {
					warnings = append(warnings, LintWarning{
						Type:        "relative_path",
						RawMatch:    pathVal,
						Suggestion:  fmt.Sprintf("Relative file path '%s' will break when called from other directories! Convert to absolute path '%s' or use parameter {{key_path}}.", pathVal, absPath),
						VariableKey: "key_path",
					})
				}
			}
		}
	}

	// 3. Check for potential raw tokens or passwords
	tokenMatches := tokenPattern.FindAllStringSubmatch(rawCmd, -1)
	for _, m := range tokenMatches {
		if len(m) < 3 {
			continue
		}
		secretVal := m[2]
		if !strings.HasPrefix(secretVal, "{{") && !strings.HasPrefix(secretVal, "$") {
			warnings = append(warnings, LintWarning{
				Type:        "raw_token",
				RawMatch:    secretVal,
				Suggestion:  "Potentially sensitive token or secret detected in plaintext. Consider parameterizing with {{token}}.",
				VariableKey: "token",
			})
		}
	}

	return warnings
}
