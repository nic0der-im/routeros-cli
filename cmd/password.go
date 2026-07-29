package cmd

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// isInteractiveTerminal reports whether stdin is a TTY suitable for prompts.
func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// readPassword reads a password either from a pipe (--password-stdin) or
// interactively from the terminal (no echo). Never prints the secret.
func readPassword(passwordStdin bool) (string, error) {
	if passwordStdin {
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("reading password from stdin: %w", err)
			}
			return "", fmt.Errorf("reading password from stdin: no input")
		}
		password := strings.TrimSpace(scanner.Text())
		if password == "" {
			return "", fmt.Errorf("empty password")
		}
		return password, nil
	}

	return promptPassword("Password")
}

func promptPassword(label string) (string, error) {
	if !isInteractiveTerminal() {
		return "", fmt.Errorf("stdin is not a terminal; use --password-stdin to pipe the password")
	}

	fmt.Fprintf(os.Stderr, "%s: ", label)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	password := strings.TrimSpace(string(bytePassword))
	if password == "" {
		return "", fmt.Errorf("empty password")
	}
	return password, nil
}

// promptLine asks a question on stderr and reads a line from stdin.
// If def is non-empty and the user presses Enter, def is returned.
func promptLine(r io.Reader, w io.Writer, label, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(w, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(w, "%s: ", label)
	}

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no input for %s", label)
	}
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return def, nil
	}
	return val, nil
}

func promptYesNo(r io.Reader, w io.Writer, label string, def bool) (bool, error) {
	defStr := "y/N"
	if def {
		defStr = "Y/n"
	}
	raw, err := promptLine(r, w, label+" ("+defStr+")", "")
	if err != nil {
		return false, err
	}
	if raw == "" {
		return def, nil
	}
	switch strings.ToLower(raw) {
	case "y", "yes", "true", "1":
		return true, nil
	case "n", "no", "false", "0":
		return false, nil
	default:
		return def, nil
	}
}

// joinHostPort builds host:port, accepting bare host or already-qualified address.
func joinHostPort(host, port string) (string, error) {
	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" {
		return "", fmt.Errorf("host is required")
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		// Already host:port
		return host, nil
	}
	// IPv6 without brackets/port
	if strings.Count(host, ":") > 1 && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	if port == "" {
		port = "8728"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid port %q", port)
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), port), nil
}
