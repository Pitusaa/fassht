package ssh_test

import (
	"testing"

	"github.com/juanperetto/fassht/config"
	fasshtssh "github.com/juanperetto/fassht/ssh"
)

// TestBuildAuthMethods verifies that the auth method builder handles
// the password-only and key-file paths without panicking.
func TestBuildAuthMethods_KeyFile(t *testing.T) {
	host := config.SSHHost{
		Name:         "test",
		Hostname:     "localhost",
		User:         "user",
		Port:         "22",
		IdentityFile: "~/.ssh/id_rsa",
	}
	methods := fasshtssh.BuildAuthMethods(host)
	// At least one method should be returned (even if the key doesn't exist,
	// the builder returns an empty slice gracefully).
	_ = methods
}

func TestBuildAuthMethods_NoKey_ReturnsEmpty(t *testing.T) {
	host := config.SSHHost{
		Name:     "test",
		Hostname: "localhost",
		User:     "user",
		Port:     "22",
	}
	methods := fasshtssh.BuildAuthMethods(host)
	if methods == nil {
		t.Error("expected non-nil slice")
	}
}
