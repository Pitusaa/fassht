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
	SSH  *gossh.Client
	SFTP *sftp.Client
}

// BuildAuthMethods constructs SSH auth methods for the given host.
// It tries the SSH agent first, then a key file if specified.
func BuildAuthMethods(host config.SSHHost) []gossh.AuthMethod {
	var methods []gossh.AuthMethod

	// Try SSH agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
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

	return methods
}

// Connect establishes an SSH+SFTP connection to the given host.
func Connect(host config.SSHHost) (*Client, error) {
	port := host.Port
	if port == "" {
		port = "22"
	}

	cfg := &gossh.ClientConfig{
		User:            host.User,
		Auth:            BuildAuthMethods(host),
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%s", host.Hostname, port)
	sshClient, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("sftp session: %w", err)
	}

	return &Client{SSH: sshClient, SFTP: sftpClient}, nil
}

// Close closes the SFTP session and SSH connection.
func (c *Client) Close() error {
	if c.SFTP != nil {
		c.SFTP.Close()
	}
	if c.SSH != nil {
		return c.SSH.Close()
	}
	return nil
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
