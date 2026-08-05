package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/nic0der-im/routeros-cli/internal/output"
)

// isInteractiveTerminal reports whether stdin is a TTY suitable for prompts.
func isInteractiveTerminal() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// readPassword reads a password either from a pipe (--password-stdin) or
// interactively from the terminal (no echo). Never prints the secret.
func readPassword(passwordStdin bool) (string, error) {
	if passwordStdin {
		return readPasswordFrom(os.Stdin)
	}

	return promptPassword("Password")
}

const maxStdinPasswordBytes = 64 * 1024

// readPasswordFrom reads exactly one password from a pipe. It removes only a
// final LF or CRLF, rejects ambiguous trailing input, and never prints it.
func readPasswordFrom(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxStdinPasswordBytes+1))
	if err != nil {
		return "", fmt.Errorf("reading password from stdin: %w", err)
	}
	if len(data) > maxStdinPasswordBytes {
		return "", fmt.Errorf("reading password from stdin: input is too long")
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("reading password from stdin: NUL byte is not allowed")
	}

	if bytes.HasSuffix(data, []byte("\n")) {
		data = data[:len(data)-1]
		if bytes.HasSuffix(data, []byte("\r")) {
			data = data[:len(data)-1]
		}
	}
	if bytes.IndexByte(data, '\n') >= 0 || bytes.IndexByte(data, '\r') >= 0 {
		return "", fmt.Errorf("reading password from stdin: extra or malformed input")
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty password")
	}
	return string(data), nil
}

const passwordStdinFlag = "password-stdin"

func readMutationPassword(verb, path string, args []string, enabled bool) (string, error) {
	return readMutationPasswordFrom(verb, path, args, enabled, os.Stdin)
}

func readMutationPasswordFrom(verb, path string, args []string, enabled bool, r io.Reader) (string, error) {
	if !enabled {
		return "", nil
	}
	if normalizePath(path) != "/user" {
		return "", fmt.Errorf("--password-stdin is only supported for generic %s user mutations", verb)
	}
	if hasPasswordArg(args) {
		return "", fmt.Errorf("--password-stdin cannot be combined with positional password=...; provide the password only on stdin")
	}
	return readPasswordFrom(r)
}

func hasPasswordArg(args []string) bool {
	for _, arg := range args {
		rest := strings.TrimSpace(arg)
		for _, prefix := range []string{"?=", "=", "?"} {
			if strings.HasPrefix(rest, prefix) {
				rest = rest[len(prefix):]
				break
			}
		}
		if i := strings.IndexByte(rest, '='); i >= 0 {
			if strings.EqualFold(strings.TrimSpace(rest[:i]), "password") {
				return true
			}
		} else if strings.EqualFold(rest, "password") {
			return true
		}
	}
	return false
}

func appendPasswordArg(args []string, password string) []string {
	if password == "" {
		return args
	}
	out := append([]string(nil), args...)
	return append(out, "=password="+password)
}

func redactAPIArgs(args []string) []string {
	if args == nil {
		return nil
	}
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = redactAPIArg(arg)
	}
	return out
}

func redactAPIArg(arg string) string {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		return trimmed
	}
	prefix := ""
	rest := trimmed
	for _, candidate := range []string{"?=", "=", "?"} {
		if strings.HasPrefix(rest, candidate) {
			prefix = candidate
			rest = rest[len(candidate):]
			break
		}
	}
	i := strings.IndexByte(rest, '=')
	if i <= 0 {
		return trimmed
	}
	key, value := rest[:i], rest[i+1:]
	return prefix + key + "=" + output.RedactValue(key, value)
}

type redactedError struct {
	err     error
	secrets []string
}

func (e *redactedError) Error() string {
	message := e.err.Error()
	for _, secret := range e.secrets {
		message = strings.ReplaceAll(message, secret, output.RedactedPlaceholder)
	}
	return message
}

func (e *redactedError) Unwrap() error { return e.err }

func secretValuesFromAPIArgs(args []string) []string {
	var secrets []string
	for _, arg := range args {
		rest := strings.TrimSpace(arg)
		for _, prefix := range []string{"?=", "=", "?"} {
			if strings.HasPrefix(rest, prefix) {
				rest = rest[len(prefix):]
				break
			}
		}
		i := strings.IndexByte(rest, '=')
		if i <= 0 || !output.IsSecretKey(rest[:i]) || rest[i+1:] == "" {
			continue
		}
		secrets = append(secrets, rest[i+1:])
	}
	return secrets
}

func redactErrorWithAPIArgs(err error, args []string) error {
	if err == nil {
		return nil
	}
	secrets := secretValuesFromAPIArgs(args)
	if len(secrets) == 0 {
		return err
	}
	return &redactedError{err: err, secrets: secrets}
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
