package cmd

import (
	"fmt"
	"strings"
)

// normalizeCloudDDNSArgs adjusts /ip/cloud set args for ROS ≥7.17 ddns-enabled enum.
//
// Rules (case-insensitive value):
//   - "no" → error (ROS rejects; use yes|auto; auto ≈ off unless Back To Home)
//   - "false" → rewritten to "auto" with a one-line tip (RouterOS accepts false oddly)
//   - other values left unchanged
//
// tip is empty when no rewrite occurred. Non-/ip/cloud paths are returned unchanged.
func normalizeCloudDDNSArgs(path string, apiArgs []string) (out []string, tip string, err error) {
	if normalizePath(path) != "/ip/cloud" {
		return apiArgs, "", nil
	}
	out = make([]string, len(apiArgs))
	copy(out, apiArgs)
	for i, a := range out {
		key, val, ok := splitAPISetArg(a)
		if !ok || !strings.EqualFold(key, "ddns-enabled") {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "no":
			return nil, "", fmt.Errorf(
				"ddns-enabled=no is invalid on RouterOS ≥7.17 (enum is yes|auto; auto = off unless Back To Home). Use: set ip/cloud ddns-enabled=auto")
		case "false":
			out[i] = "=ddns-enabled=auto"
			tip = "note: ddns-enabled=false normalized to auto (ROS ≥7.17 off unless Back To Home)"
		}
	}
	return out, tip, nil
}

// splitAPISetArg parses =key=value API set args.
func splitAPISetArg(a string) (key, val string, ok bool) {
	if !strings.HasPrefix(a, "=") || strings.HasPrefix(a, "?") {
		return "", "", false
	}
	body := strings.TrimPrefix(a, "=")
	idx := strings.IndexByte(body, '=')
	if idx <= 0 {
		return "", "", false
	}
	return body[:idx], body[idx+1:], true
}
