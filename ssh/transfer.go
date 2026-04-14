package ssh

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileEntry represents a file on the remote server.
type FileEntry struct {
	Path string // absolute remote path
	Name string // base name
}

// TempFilePath returns a unique local temp path for a given remote file path.
func TempFilePath(remotePath string) string {
	hash := sha256.Sum256([]byte(remotePath))
	name := filepath.Base(remotePath)
	return filepath.Join(os.TempDir(), fmt.Sprintf("fassht_%x_%s", hash[:6], name))
}

// ListFiles runs `find <basePath> -maxdepth 5 -type f` via SSH exec
// and returns the results as FileEntry slice.
func (c *Client) ListFiles(basePath string) ([]FileEntry, error) {
	session, err := c.SSH.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	cmd := fmt.Sprintf("find %s -maxdepth 5 -type f 2>/dev/null", basePath)
	if err := session.Run(cmd); err != nil {
		// find returns exit code 1 on permission errors — that's OK
		if out.Len() == 0 {
			return nil, fmt.Errorf("list files: %w", err)
		}
	}

	var entries []FileEntry
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, FileEntry{
			Path: line,
			Name: filepath.Base(line),
		})
	}
	return entries, nil
}

// Download fetches a remote file via SFTP and writes it to localPath.
func (c *Client) Download(remotePath, localPath string) error {
	remote, err := c.SFTP.Open(remotePath)
	if err != nil {
		return fmt.Errorf("sftp open %s: %w", remotePath, err)
	}
	defer remote.Close()

	local, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local file %s: %w", localPath, err)
	}
	defer local.Close()

	if _, err := io.Copy(local, remote); err != nil {
		return fmt.Errorf("download copy: %w", err)
	}
	return nil
}

// Upload writes a local file to a remote path via SFTP.
func (c *Client) Upload(localPath, remotePath string) error {
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local file %s: %w", localPath, err)
	}
	defer local.Close()

	remote, err := c.SFTP.Create(remotePath)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", remotePath, err)
	}
	defer remote.Close()

	if _, err := io.Copy(remote, local); err != nil {
		return fmt.Errorf("upload copy: %w", err)
	}
	return nil
}

// CopyFile copies src to dst on the local filesystem.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
