package cmd

import (
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/nic0der-im/routeros-cli/internal/session"
)

// parseAPIArgs converts CLI-friendly args into RouterOS API args.
//
// Accepted forms:
//
//	key=value        → =key=value
//	.key=value       → =.key=value
//	?=key=value      → ?=key=value  (query/filter)
//	=key=value       → passed through
//	?key=value       → ?key=value
func parseAPIArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		switch {
		case strings.HasPrefix(a, "=") || strings.HasPrefix(a, "?"):
			out = append(out, a)
		case strings.HasPrefix(a, "?="):
			out = append(out, a)
		default:
			out = append(out, "="+a)
		}
	}
	return out
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// collapse duplicate slashes except leading
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return strings.TrimSuffix(path, "/")
}

func pathCommand(path, action string) string {
	return normalizePath(path) + "/" + action
}

func extractCreatedID(result *client.Result) string {
	if result == nil {
		return ""
	}
	for i := len(result.Sentences) - 1; i >= 0; i-- {
		s := result.Sentences[i]
		if id := s["ret"]; id != "" {
			return id
		}
		if id := s[".id"]; id != "" {
			return id
		}
	}
	return ""
}

func recordCreateChange(a *App, deviceName, command string, args []string, result *client.Result) error {
	id := extractCreatedID(result)
	if id == "" {
		return nil
	}
	inv := session.BuildInverse(command, args, id)
	if len(inv) == 0 {
		return nil
	}
	return a.recordSafeChange(deviceName, session.Change{
		Command: command,
		Args:    args,
		Inverse: inv,
		Note:    "created " + id,
	})
}

func recordIDChange(a *App, deviceName, command string, args []string, inverse []string, note string) error {
	if len(inverse) == 0 {
		return nil
	}
	return a.recordSafeChange(deviceName, session.Change{
		Command: command,
		Args:    args,
		Inverse: inverse,
		Note:    note,
	})
}

// findIDArg extracts .id= value from parsed args.
func findIDArg(args []string) string {
	for _, a := range args {
		if strings.HasPrefix(a, "=.id=") {
			return strings.TrimPrefix(a, "=.id=")
		}
		if strings.HasPrefix(a, "=id=") {
			return strings.TrimPrefix(a, "=id=")
		}
	}
	return ""
}
