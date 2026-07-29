package winbox

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultWinboxDirs returns likely Winbox data directories for the current OS,
// including Wine prefixes when present on Unix.
func DefaultWinboxDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	var dirs []string
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs, filepath.Join(home, "Library", "Application Support", "MikroTik", "WinBox"))
		dirs = append(dirs, filepath.Join(home, "Library", "Application Support", "MikroTik", "Winbox"))
		dirs = append(dirs, wineWinboxDirs(home)...)
	case "linux":
		dirs = append(dirs, filepath.Join(home, ".local", "share", "MikroTik", "WinBox"))
		dirs = append(dirs, filepath.Join(home, ".local", "share", "MikroTik", "Winbox"))
		dirs = append(dirs, wineWinboxDirs(home)...)
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			dirs = append(dirs, filepath.Join(appData, "MikroTik", "WinBox"))
			dirs = append(dirs, filepath.Join(appData, "MikroTik", "Winbox"))
		}
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "MikroTik", "WinBox"))
		}
	default:
		dirs = append(dirs, filepath.Join(home, ".local", "share", "MikroTik", "WinBox"))
		dirs = append(dirs, wineWinboxDirs(home)...)
	}
	return uniqueStrings(dirs)
}

func wineWinboxDirs(home string) []string {
	prefixes := []string{
		filepath.Join(home, ".wine"),
		filepath.Join(home, ".local", "share", "wine"),
	}
	var out []string
	for _, p := range prefixes {
		// Common Wine user profile layouts.
		candidates := []string{
			filepath.Join(p, "drive_c", "users", filepath.Base(home), "AppData", "Roaming", "MikroTik", "Winbox"),
			filepath.Join(p, "drive_c", "users", filepath.Base(home), "AppData", "Roaming", "MikroTik", "WinBox"),
			filepath.Join(p, "drive_c", "users", "crossover", "AppData", "Roaming", "MikroTik", "Winbox"),
			filepath.Join(p, "drive_c", "users", "crossover", "AppData", "Roaming", "MikroTik", "WinBox"),
		}
		out = append(out, candidates...)
	}
	return out
}

// DefaultCDBPaths returns candidate Addresses.cdb paths (Winbox 4).
func DefaultCDBPaths() []string {
	var paths []string
	for _, dir := range DefaultWinboxDirs() {
		paths = append(paths,
			filepath.Join(dir, "Addresses.cdb"),
			filepath.Join(dir, "addresses.cdb"),
		)
	}
	return uniqueStrings(paths)
}

// DefaultWBXPaths returns candidate addresses.WBX paths (Winbox 3).
func DefaultWBXPaths() []string {
	var paths []string
	for _, dir := range DefaultWinboxDirs() {
		paths = append(paths,
			filepath.Join(dir, "addresses.WBX"),
			filepath.Join(dir, "Addresses.WBX"),
			filepath.Join(dir, "addresses.wbx"),
			filepath.Join(dir, "Addresses.wbx"),
		)
	}
	return uniqueStrings(paths)
}

// FindDefaultFile looks for a Winbox address book on the current OS.
// Prefers Winbox 4 CDB, then Winbox 3 WBX. Returns ("", error) when none exist.
func FindDefaultFile() (path string, err error) {
	for _, p := range DefaultCDBPaths() {
		if fileExists(p) {
			return p, nil
		}
	}
	for _, p := range DefaultWBXPaths() {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Winbox address book found; tried CDB then WBX under default MikroTik/WinBox paths (use --file)")
}

// ParseFile reads path and dispatches to CDB or WBX based on extension / magic.
func ParseFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty winbox file: %s", path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".cdb":
		return ParseCDB(data)
	case ".wbx":
		return ParseWBX(data)
	}
	// Magic sniff.
	if len(data) >= 4 {
		if data[0] == 0x0d && data[1] == 0xf0 && data[2] == 0x1d && data[3] == 0xc0 {
			return ParseCDB(data)
		}
		if data[0] == 0x0f && data[1] == 0x10 && data[2] == 0xc0 && data[3] == 0xbe {
			return ParseWBX(data)
		}
	}
	// Last resort: try CDB then WBX.
	if entries, err := ParseCDB(data); err == nil && len(entries) > 0 {
		return entries, nil
	}
	return ParseWBX(data)
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
