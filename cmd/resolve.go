package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/nic0der-im/routeros-cli/internal/apperr"
	"github.com/nic0der-im/routeros-cli/internal/client"
	"github.com/spf13/cobra"
)

// commentTargetFlag is the cobra flag name for exact comment→.id resolution
// on firewall filter/mangle mutate paths.
const commentTargetFlag = "comment"

// supportsCommentAsID reports whether path may be targeted by --comment (B5).
// Only filter and mangle; address-list keeps list+address semantic keys.
func supportsCommentAsID(path string) bool {
	switch normalizePath(path) {
	case "/ip/firewall/filter", "/ip/firewall/mangle":
		return true
	default:
		return false
	}
}

// attachCommentTargetFlag registers --comment as an exact-match rule selector
// (alternative to .id) for filter/mangle mutate verbs.
func attachCommentTargetFlag(cmd *cobra.Command) {
	cmd.Flags().String(commentTargetFlag, "", "exact rule comment for firewall/filter or firewall/mangle (alternative to .id)")
}

func getCommentTarget(cmd *cobra.Command) string {
	v, err := cmd.Flags().GetString(commentTargetFlag)
	if err != nil {
		return ""
	}
	return v
}

// resolveIDByComment prints the path and returns .id for the row whose comment
// field equals comment exactly (case-sensitive value; key lookup is
// case-insensitive). Zero matches → not_found; multiple → conflict.
func resolveIDByComment(ctx context.Context, c client.Client, path, comment string) (string, error) {
	if comment == "" {
		return "", apperr.New(apperr.KindConfig, "comment must be non-empty")
	}
	rows, err := fetchAllRows(ctx, c, path)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, row := range rows {
		if rowFieldCI(row, "comment") != comment {
			continue
		}
		if id := rowFieldID(row); id != "" {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", apperr.New(apperr.KindNotFound,
			fmt.Sprintf("%s: no rule with comment %q", normalizePath(path), comment))
	case 1:
		return matches[0], nil
	default:
		return "", apperr.New(apperr.KindConflict,
			fmt.Sprintf("%s: comment %q matches %d rules (%s); use unique comments or .id",
				normalizePath(path), comment, len(matches), strings.Join(matches, ", "))).
			WithSuggestedAction("use unique comments or specify .id=*N")
	}
}

// resolveMutateTargetID returns an explicit .id, or resolves --comment for
// filter/mangle. id and comment are mutually exclusive.
func resolveMutateTargetID(ctx context.Context, c client.Client, path, id, comment string) (string, error) {
	if id != "" && comment != "" {
		return "", apperr.New(apperr.KindConfig, "specify either --id/.id or --comment, not both")
	}
	if id != "" {
		return id, nil
	}
	if comment == "" {
		return "", apperr.New(apperr.KindConfig, "requires --id/.id or --comment")
	}
	if !supportsCommentAsID(path) {
		return "", apperr.New(apperr.KindConfig,
			fmt.Sprintf("--comment targeting is only supported for firewall/filter and firewall/mangle, not %s",
				normalizePath(path)))
	}
	return resolveIDByComment(ctx, c, path, comment)
}

// ensureIDArg ensures apiArgs contain =.id=<id>, replacing any prior .id/id.
func ensureIDArg(apiArgs []string, id string) []string {
	out := make([]string, 0, len(apiArgs)+1)
	for _, a := range apiArgs {
		if strings.HasPrefix(a, "=.id=") || strings.HasPrefix(a, "=id=") {
			continue
		}
		out = append(out, a)
	}
	return append([]string{"=.id=" + id}, out...)
}

func rowFieldCI(row map[string]string, key string) string {
	if row == nil {
		return ""
	}
	if v, ok := row[key]; ok {
		return v
	}
	want := strings.ToLower(key)
	for k, v := range row {
		if strings.ToLower(k) == want {
			return v
		}
	}
	return ""
}

func rowFieldID(row map[string]string) string {
	if id := row[".id"]; id != "" {
		return id
	}
	for k, v := range row {
		if strings.EqualFold(k, ".id") && v != "" {
			return v
		}
	}
	return ""
}
