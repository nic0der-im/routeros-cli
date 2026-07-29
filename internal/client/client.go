// Package client provides an abstraction over the go-routeros library for
// communicating with RouterOS devices via the native API protocol.
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-routeros/routeros/v3"
)

// defaultTimeout is applied when ConnectConfig.Timeout is zero.
const defaultTimeout = 10 * time.Second

// Result wraps a RouterOS command response as a slice of sentence maps.
type Result struct {
	Sentences []map[string]string
}

// Client defines the interface for communicating with a RouterOS device.
type Client interface {
	Run(ctx context.Context, command string, args ...string) (*Result, error)
	Close() error
}

// ConnectConfig holds parameters for connecting to a RouterOS device.
type ConnectConfig struct {
	Address            string
	Username           string
	Password           string
	UseTLS             bool
	InsecureSkipVerify bool
	CACertPath         string
	Timeout            time.Duration
}

// RouterOSClient wraps go-routeros for real device communication.
type RouterOSClient struct {
	conn *routeros.Client
}

// Connect establishes a connection to a RouterOS device. If cfg.UseTLS is
// true, a TLS connection is made with the provided TLS settings; otherwise a
// plain-text connection is established. A default timeout of 10 seconds is
// used when cfg.Timeout is zero.
func Connect(ctx context.Context, cfg ConnectConfig) (*RouterOSClient, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var (
		conn *routeros.Client
		err  error
	)

	if cfg.UseTLS {
		tlsCfg, tlsErr := buildTLSConfig(cfg)
		if tlsErr != nil {
			return nil, fmt.Errorf("building TLS config: %w", tlsErr)
		}
		conn, err = routeros.DialTLSContext(ctx, cfg.Address, cfg.Username, cfg.Password, tlsCfg)
	} else {
		conn, err = routeros.DialContext(ctx, cfg.Address, cfg.Username, cfg.Password)
	}

	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", cfg.Address, err)
	}

	return &RouterOSClient{conn: conn}, nil
}

func buildTLSConfig(cfg ConnectConfig) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // intentional for self-signed lab routers
		MinVersion:         tls.VersionTLS12,
	}

	if cfg.CACertPath != "" {
		pem, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert %q: %w", cfg.CACertPath, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("parsing CA cert %q: no certificates found", cfg.CACertPath)
		}
		tlsCfg.RootCAs = pool
	}

	return tlsCfg, nil
}

// Run executes a RouterOS command and returns the result. Arguments that do
// not already start with "=" or "?" are automatically prefixed with "=".
func (c *RouterOSClient) Run(ctx context.Context, command string, args ...string) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("running %s: %w", command, err)
	}

	cmdArgs := make([]string, 0, 1+len(args))
	cmdArgs = append(cmdArgs, command)

	for _, arg := range args {
		if !strings.HasPrefix(arg, "=") && !strings.HasPrefix(arg, "?") {
			arg = "=" + arg
		}
		cmdArgs = append(cmdArgs, arg)
	}

	reply, err := c.conn.RunArgs(cmdArgs)
	if err != nil {
		return nil, fmt.Errorf("running %s: %w", command, err)
	}

	sentences := make([]map[string]string, 0, len(reply.Re)+1)
	for _, sentence := range reply.Re {
		sentences = append(sentences, sentence.Map)
	}
	// /add replies put the created .id in !done =ret=*N (not in !re).
	if reply.Done != nil {
		if ret, ok := reply.Done.Map["ret"]; ok && ret != "" {
			sentences = append(sentences, map[string]string{"ret": ret, ".id": ret})
		}
	}

	return &Result{Sentences: sentences}, nil
}

// Close closes the underlying connection to the RouterOS device.
func (c *RouterOSClient) Close() error {
	c.conn.Close() //nolint:errcheck // routeros.Client.Close is void
	return nil
}
