package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/juanperetto/fassht/config"
)

func writeTempSSHConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSSHHosts_ParsesHosts(t *testing.T) {
	sshCfg := `
Host myserver
    HostName 192.168.1.10
    User deploy
    Port 2222
    IdentityFile ~/.ssh/id_rsa

Host prod
    HostName prod.example.com
    User ubuntu
`
	path := writeTempSSHConfig(t, sshCfg)
	hosts, err := config.LoadSSHHostsFrom(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].Name != "myserver" {
		t.Errorf("expected 'myserver', got '%s'", hosts[0].Name)
	}
	if hosts[0].Hostname != "192.168.1.10" {
		t.Errorf("expected '192.168.1.10', got '%s'", hosts[0].Hostname)
	}
	if hosts[0].User != "deploy" {
		t.Errorf("expected 'deploy', got '%s'", hosts[0].User)
	}
	if hosts[0].Port != "2222" {
		t.Errorf("expected '2222', got '%s'", hosts[0].Port)
	}
	if hosts[0].IdentityFile != "~/.ssh/id_rsa" {
		t.Errorf("expected '~/.ssh/id_rsa', got '%s'", hosts[0].IdentityFile)
	}
}

func TestAppendSSHHost_AddsNewEntry(t *testing.T) {
	path := writeTempSSHConfig(t, "")
	host := config.SSHHost{
		Name:     "newhost",
		Hostname: "10.0.0.1",
		User:     "root",
		Port:     "22",
	}
	if err := config.AppendSSHHostTo(path, host); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hosts, err := config.LoadSSHHostsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Name != "newhost" {
		t.Errorf("expected newhost to be saved, got %v", hosts)
	}
}
