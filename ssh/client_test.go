package ssh_test

import (
	"testing"

	"github.com/Pitusaa/fassht/config"
	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

func TestBuildAuthMethods_KeyFile(t *testing.T) {
	host := config.SSHHost{
		Name:         "test",
		Hostname:     "localhost",
		User:         "user",
		Port:         "22",
		IdentityFile: "~/.ssh/id_rsa",
	}
	methods, agentConn := fasshtssh.BuildAuthMethods(host)
	if agentConn != nil {
		defer agentConn.Close()
	}
	_ = methods
}

func TestBuildAuthMethods_NoKey_ReturnsEmpty(t *testing.T) {
	host := config.SSHHost{
		Name:     "test",
		Hostname: "localhost",
		User:     "user",
		Port:     "22",
	}
	methods, agentConn := fasshtssh.BuildAuthMethods(host)
	if agentConn != nil {
		defer agentConn.Close()
	}
	if methods == nil {
		t.Error("expected non-nil slice")
	}
}
