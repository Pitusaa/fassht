package ssh

import (
	"fmt"
	"strings"
)

// PermissionInfo describes the access options for a remote file.
type PermissionInfo struct {
	Readable  bool // connected user can read the file
	Writable  bool // connected user can write the file
	CanChmod  bool // connected user owns the file and can chmod without sudo
	CanSudo   bool // sudo binary is available on the remote host
	NeedsSudo bool // sudo requires a password (not passwordless)
}

// CheckWritePermission runs a single SSH command to determine whether the
// connected user can write to remotePath and what remediation options exist.
func (c *Client) CheckWritePermission(remotePath string) (PermissionInfo, error) {
	session, err := c.SSH.NewSession()
	if err != nil {
		return PermissionInfo{}, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	q := shellQuote(remotePath)
	// Outputs five 0/1 integers: readable writable owns sudo_avail sudo_passwordless
	script := fmt.Sprintf(
		`r=0; w=0; o=0; sa=0; so=0
	test -r %s && r=1
	test -w %s && w=1
	[ "$(stat -c %%u %s 2>/dev/null)" = "$(id -u)" ] && o=1
	command -v sudo >/dev/null 2>&1 && sa=1
	sudo -n true 2>/dev/null && so=1
	printf '%%d %%d %%d %%d %%d\n' "$r" "$w" "$o" "$sa" "$so"`,
		q, q, q,
	)

	var out strings.Builder
	session.Stdout = &out
	_ = session.Run(script) // non-zero exit is expected when sudo -n fails

	parts := strings.Fields(strings.TrimSpace(out.String()))
	if len(parts) != 5 {
		return PermissionInfo{}, fmt.Errorf("unexpected permission check output: %q", out.String())
	}
	bit := func(i int) bool { return parts[i] == "1" }
	return PermissionInfo{
		Readable:  bit(0),
		Writable:  bit(1),
		CanChmod:  bit(2),
		CanSudo:   bit(3),
		NeedsSudo: bit(3) && !bit(4),
	}, nil
}

// ChmodWritable makes remotePath user-writable via chmod without sudo.
func (c *Client) ChmodWritable(remotePath string) error {
	return c.runChmod(remotePath, false, "")
}

// SudoChmodWritable makes remotePath user-writable via sudo chmod.
// Pass an empty password for passwordless sudo.
func (c *Client) SudoChmodWritable(remotePath, password string) error {
	return c.runChmod(remotePath, true, password)
}

func (c *Client) runChmod(remotePath string, useSudo bool, sudoPassword string) error {
	session, err := c.SSH.NewSession()
	if err != nil {
		return fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	q := shellQuote(remotePath)
	var cmd string
	if useSudo {
		cmd = "sudo -S chmod u+rw " + q
		session.Stdin = strings.NewReader(sudoPassword + "\n")
	} else {
		cmd = "chmod u+rw " + q
	}

	var errBuf strings.Builder
	session.Stderr = &errBuf
	if err := session.Run(cmd); err != nil {
		if msg := strings.TrimSpace(errBuf.String()); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return fmt.Errorf("chmod u+w: %w", err)
	}
	return nil
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
