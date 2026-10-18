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
	// Regex matching KEY=http(s)://...
	keyValueURLPattern = regexp.MustCompile(`\b([a-zA-Z0-9_-]+)=(https?://[^\s"']+)`)
	// Regex matching build defines e.g. --dart-define KEY=VAL or --env KEY=VAL
	definePattern = regexp.MustCompile(`(--dart-define|--env|-e|--define)[= ]+([a-zA-Z0-9_-]+)=([^"'\s]+)`)
	// Regex matching standalone URLs
	urlPattern = regexp.MustCompile(`\bhttps?://[^\s"']+`)
)

type LintWarning struct {
	Type            string // "key_value_url", "define_var", "raw_url", "relative_path", "raw_ip", "raw_token"
	TargetSpan      string // The exact substring in the command to replace
	ReplacementSpan string // The replacement substring (e.g. URL={{url}})
	RawMatch        string // The extracted value to persist (e.g. https://...)
	Suggestion      string
	VariableKey     string
}

// Analyze inspects a raw command string and flags security and portability issues.
func Analyze(rawCmd string) []LintWarning {
	var warnings []LintWarning
	replacedSpans := make(map[string]bool)

	// 1. Check for KEY=https://... pairs (e.g. URL=https://tabletserveroci...)
	for _, m := range keyValueURLPattern.FindAllStringSubmatch(rawCmd, -1) {
		if len(m) < 3 {
			continue
		}
		key := m[1]
		val := m[2]
		if strings.HasPrefix(val, "{{") || strings.HasPrefix(val, "$") {
			continue
		}
		varKey := sanitizeVarName(key)
		targetSpan := m[0]
		replacementSpan := fmt.Sprintf("%s={{%s}}", key, varKey)

		warnings = append(warnings, LintWarning{
			Type:            "key_value_url",
			TargetSpan:      targetSpan,
			ReplacementSpan: replacementSpan,
			RawMatch:        val,
			Suggestion:      fmt.Sprintf("Endpoint URL definition '%s' detected. Consider parameterizing with {{%s}}.", targetSpan, varKey),
			VariableKey:     varKey,
		})
		replacedSpans[targetSpan] = true
	}

	// 2. Check for build define flags e.g. --dart-define KEY=VAL
	for _, m := range definePattern.FindAllStringSubmatch(rawCmd, -1) {
		if len(m) < 4 {
			continue
		}
		key := m[2]
		val := m[3]
		targetSpan := fmt.Sprintf("%s=%s", key, val)
		if replacedSpans[targetSpan] || strings.HasPrefix(val, "{{") || strings.HasPrefix(val, "$") {
			continue
		}
		varKey := sanitizeVarName(key)
		replacementSpan := fmt.Sprintf("%s={{%s}}", key, varKey)

		warnings = append(warnings, LintWarning{
			Type:            "define_var",
			TargetSpan:      targetSpan,
			ReplacementSpan: replacementSpan,
			RawMatch:        val,
			Suggestion:      fmt.Sprintf("Build configuration '%s' detected. Consider parameterizing with {{%s}}.", targetSpan, varKey),
			VariableKey:     varKey,
		})
		replacedSpans[targetSpan] = true
	}

	// 3. Check for standalone URLs (not part of an already flagged KEY=URL)
	for _, match := range urlPattern.FindAllString(rawCmd, -1) {
		alreadyCovered := false
		for span := range replacedSpans {
			if strings.Contains(span, match) {
				alreadyCovered = true
				break
			}
		}
		if !alreadyCovered && !strings.Contains(match, "{{") {
			warnings = append(warnings, LintWarning{
				Type:            "raw_url",
				TargetSpan:      match,
				ReplacementSpan: "{{url}}",
				RawMatch:        match,
				Suggestion:      fmt.Sprintf("Raw URL '%s' detected. Consider parameterizing with {{url}}.", match),
				VariableKey:     "url",
			})
		}
	}

	// 4. Check for raw IPs
	for _, match := range ipPattern.FindAllString(rawCmd, -1) {
		if ip := net.ParseIP(match); ip != nil && !ip.IsLoopback() {
			warnings = append(warnings, LintWarning{
				Type:            "raw_ip",
				TargetSpan:      match,
				ReplacementSpan: "{{server_ip}}",
				RawMatch:        match,
				Suggestion:      fmt.Sprintf("Raw public IP '%s' detected. Consider parameterizing with {{server_ip}} to keep secrets out of Git.", match),
				VariableKey:     "server_ip",
			})
		}
	}

	// 5. Check for relative file flags (-i "chave.key", etc.)
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
						Type:            "relative_path",
						TargetSpan:      pathVal,
						ReplacementSpan: "{{key_path}}",
						RawMatch:        pathVal,
						Suggestion:      fmt.Sprintf("Relative file path '%s' will break when called from other directories! Convert to absolute path '%s' or use parameter {{key_path}}.", pathVal, absPath),
						VariableKey:     "key_path",
					})
				}
			}
		}
	}

	// 6. Check for potential raw tokens or passwords
	tokenMatches := tokenPattern.FindAllStringSubmatch(rawCmd, -1)
	for _, m := range tokenMatches {
		if len(m) < 3 {
			continue
		}
		secretVal := m[2]
		if !strings.HasPrefix(secretVal, "{{") && !strings.HasPrefix(secretVal, "$") {
			warnings = append(warnings, LintWarning{
				Type:            "raw_token",
				TargetSpan:      secretVal,
				ReplacementSpan: "{{token}}",
				RawMatch:        secretVal,
				Suggestion:      "Potentially sensitive token or secret detected in plaintext. Consider parameterizing with {{token}}.",
				VariableKey:     "token",
			})
		}
	}

	return warnings
}

func sanitizeVarName(key string) string {
	clean := strings.ToLower(key)
	clean = strings.ReplaceAll(clean, "-", "_")
	var sb strings.Builder
	for _, r := range clean {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	res := sb.String()
	if res == "" {
		return "var"
	}
	return res
}
