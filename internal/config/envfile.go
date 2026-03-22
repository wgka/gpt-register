package config

import (
	"os"
	"sort"
	"strings"
)

func UpsertEnvFile(path string, updates map[string]string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	lines := splitEnvLines(string(content))
	seen := map[string]bool{}
	output := make([]string, 0, len(lines)+len(updates)+4)

	for _, line := range lines {
		key, ok := parseEnvAssignmentKey(line)
		if !ok {
			output = append(output, line)
			continue
		}

		value, tracked := updates[key]
		if !tracked {
			output = append(output, line)
			continue
		}
		if seen[key] {
			continue
		}

		output = append(output, formatEnvAssignment(key, value))
		seen[key] = true
	}

	missingKeys := make([]string, 0, len(updates))
	for key := range updates {
		if !seen[key] {
			missingKeys = append(missingKeys, key)
		}
	}
	sort.Strings(missingKeys)
	if len(missingKeys) > 0 && len(output) > 0 && strings.TrimSpace(output[len(output)-1]) != "" {
		output = append(output, "")
	}
	for _, key := range missingKeys {
		output = append(output, formatEnvAssignment(key, updates[key]))
	}

	result := strings.Join(output, "\n")
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return os.WriteFile(path, []byte(result), 0o644)
}

func splitEnvLines(content string) []string {
	if content == "" {
		return []string{}
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimRight(content, "\n")
	if content == "" {
		return []string{}
	}
	return strings.Split(content, "\n")
}

func parseEnvAssignmentKey(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	}
	key, _, ok := strings.Cut(trimmed, "=")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return "", false
	}
	return key, true
}

func formatEnvAssignment(key, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return key + "="
	}
	if strings.ContainsAny(value, " \t#") {
		value = strings.ReplaceAll(value, `\`, `\\`)
		value = strings.ReplaceAll(value, `"`, `\"`)
		return key + `="` + value + `"`
	}
	return key + "=" + value
}
