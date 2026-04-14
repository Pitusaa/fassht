package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/juanperetto/fassht/config"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client wraps an SSH connection and an SFTP session.
type Client struct {
	SSH       *gossh.Client
	SFTP      *sftp.Client
	agentConn net.Conn // kept open for agent-based auth callbacks
}

// BuildAuthMethods constructs SSH auth methods for the given host.
// It tries the SSH agent first, then a key file if specified.
// agentConn is non-nil when an SSH agent connection was established;
// callers must close it when done.
func BuildAuthMethods(host config.SSHHost) ([]gossh.AuthMethod, net.Conn) {
	var methods []gossh.AuthMethod
	var agentConn net.Conn

	// Try SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentConn = conn
			methods = append(methods, gossh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
	}

	// Try identity file
	if host.IdentityFile != "" {
		keyPath := expandTilde(host.IdentityFile)
		key, err := os.ReadFile(keyPath)
		if err == nil {
			signer, err := gossh.ParsePrivateKey(key)
			if err == nil {
				methods = append(methods, gossh.PublicKeys(signer))
			}
		}
	}

	return methods, agentConn
}

// Connect establishes an SSH+SFTP connection to the given host.
func Connect(host config.SSHHost) (*Client, error) {
	if host.Hostname == "" {
		return nil, fmt.Errorf("ssh hostname is required")
	}
	if host.User == "" {
		return nil, fmt.Errorf("ssh user is required")
	}

	port := host.Port
	if port == "" {
		port = "22"
	}

	methods, agentConn := BuildAuthMethods(host)
	cfg := &gossh.ClientConfig{
		User:            host.User,
		Auth:            methods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // TODO: replace with known_hosts in a future iteration
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", host.Hostname, port)
	sshClient, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		if agentConn != nil {
			agentConn.Close()
		}
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		if agentConn != nil {
			agentConn.Close()
		}
		return nil, fmt.Errorf("sftp session: %w", err)
	}

	return &Client{SSH: sshClient, SFTP: sftpClient, agentConn: agentConn}, nil
}

// Close closes the SFTP session, SSH connection, and SSH agent connection.
func (c *Client) Close() error {
	var firstErr error
	if c.SFTP != nil {
		if err := c.SFTP.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.SSH != nil {
		if err := c.SSH.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.agentConn != nil {
		if err := c.agentConn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
