package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pitusaa/fassht/config"
	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client wraps an SSH connection and an SFTP session.
type Client struct {
	SSH       *gossh.Client
	SFTP      *sftp.Client
	agentConn net.Conn // kept open for agent-based auth callbacks
}

// BuildAuthMethods constructs SSH auth methods for the given host.
// All available signers (agent + key files) are combined into a single
// PublicKeys call to avoid the SSH protocol marking publickey as exhausted
// after the agent returns zero keys.
// agentConn is non-nil when an SSH agent connection was established;
// callers must close it when done.
func BuildAuthMethods(host config.SSHHost) ([]gossh.AuthMethod, net.Conn) {
	var signers []gossh.Signer
	var agentConn net.Conn

	// Collect signers from SSH agent (if running and has identities)
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			agentConn = conn
			if agentSigners, err := agent.NewClient(conn).Signers(); err == nil {
				signers = append(signers, agentSigners...)
			}
		}
	}

	// Collect signer from explicit IdentityFile
	if host.IdentityFile != "" {
		if s := loadKeySigner(expandTilde(host.IdentityFile)); s != nil {
			signers = append(signers, s)
		}
	}

	// Fall back to default key locations when no IdentityFile is set
	if host.IdentityFile == "" {
		home, _ := os.UserHomeDir()
		for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
			if s := loadKeySigner(filepath.Join(home, ".ssh", name)); s != nil {
				signers = append(signers, s)
				break
			}
		}
	}

	if len(signers) == 0 {
		return nil, agentConn
	}
	return []gossh.AuthMethod{gossh.PublicKeys(signers...)}, agentConn
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
	hostKeyCallback, err := buildHostKeyCallback()
	if err != nil && agentConn != nil {
		agentConn.Close()
		return nil, err
	}
	cfg := &gossh.ClientConfig{
		User:            host.User,
		Auth:            methods,
		HostKeyCallback: hostKeyCallback,
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

// loadKeySigner reads a private key file and returns a signer, or nil if it fails.
func loadKeySigner(path string) gossh.Signer {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	signer, err := gossh.ParsePrivateKey(key)
	if err != nil {
		return nil
	}
	return signer
}

// buildHostKeyCallback returns a callback that verifies the server's host key
// against ~/.ssh/known_hosts. If the file does not exist, it falls back to
// InsecureIgnoreHostKey with a warning — matching the behaviour of OpenSSH
// when StrictHostKeyChecking=no is set for a first-time connection.
func buildHostKeyCallback() (gossh.HostKeyCallback, error) {
	home, _ := os.UserHomeDir()
	knownHostsPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return gossh.InsecureIgnoreHostKey(), nil //nolint:gosec
	}
	cb, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return cb, nil
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
