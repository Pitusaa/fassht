package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gossh "github.com/kevinburke/ssh_config"
)

// SSHHost represents a single Host block from ~/.ssh/config.
type SSHHost struct {
	Name         string
	Hostname     string
	User         string
	Port         string
	IdentityFile string
}

// DefaultSSHConfigPath returns the user's default SSH config path.
func DefaultSSHConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

// LoadSSHHostsFrom parses an SSH config file and returns all Host entries
// (skipping the wildcard "*" entry).
func LoadSSHHostsFrom(path string) ([]SSHHost, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg, err := gossh.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("parse ssh config: %w", err)
	}

	var hosts []SSHHost
	for _, host := range cfg.Hosts {
		if len(host.Patterns) == 0 {
			continue
		}
		name := host.Patterns[0].String()
		if name == "*" {
			continue
		}
		hostname, _ := cfg.Get(name, "HostName")
		user, _ := cfg.Get(name, "User")
		port, _ := cfg.Get(name, "Port")
		identityFile, _ := cfg.Get(name, "IdentityFile")
		hosts = append(hosts, SSHHost{
			Name:         name,
			Hostname:     hostname,
			User:         user,
			Port:         port,
			IdentityFile: identityFile,
		})
	}
	return hosts, nil
}

// AppendSSHHostTo appends a new Host block to the SSH config file at path.
func AppendSSHHostTo(path string, host SSHHost) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nHost %s\n", host.Name))
	sb.WriteString(fmt.Sprintf("    HostName %s\n", host.Hostname))
	if host.User != "" {
		sb.WriteString(fmt.Sprintf("    User %s\n", host.User))
	}
	if host.Port != "" && host.Port != "22" {
		sb.WriteString(fmt.Sprintf("    Port %s\n", host.Port))
	}
	if host.IdentityFile != "" {
		sb.WriteString(fmt.Sprintf("    IdentityFile %s\n", host.IdentityFile))
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open ssh config for append: %w", err)
	}
	defer f.Close()
	_, err = f.WriteString(sb.String())
	return err
}

// LoadSSHHosts loads from the default SSH config path.
func LoadSSHHosts() ([]SSHHost, error) {
	return LoadSSHHostsFrom(DefaultSSHConfigPath())
}

// AppendSSHHost appends to the default SSH config path.
func AppendSSHHost(host SSHHost) error {
	return AppendSSHHostTo(DefaultSSHConfigPath(), host)
}
