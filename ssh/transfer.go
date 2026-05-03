package ssh

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

// SearchFiles runs `find <basePath> -iname "*query*" -type f` on the remote
// server and returns up to 200 matches. The query is sanitized to safe
// filename characters before being embedded in the shell command.
func (c *Client) SearchFiles(basePath, query string) ([]FileEntry, error) {
	session, err := c.SSH.NewSession()
	if err != nil {
		return nil, fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var out bytes.Buffer
	session.Stdout = &out
	safe := sanitizeQuery(query)
	cmd := fmt.Sprintf("find %s -iname '*%s*' -type f 2>/dev/null | head -200", basePath, safe)
	if err := session.Run(cmd); err != nil {
		if out.Len() == 0 {
			return nil, fmt.Errorf("search files: %w", err)
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

// sanitizeQuery keeps only characters that are safe to embed inside a shell
// single-quoted glob pattern (alphanumeric, dot, dash, underscore, space).
func sanitizeQuery(q string) string {
	var sb strings.Builder
	for _, r := range q {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' || r == ' ' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// Download fetches a remote file via SFTP and writes it to localPath.
func (c *Client) Download(remotePath, localPath string) error {
	remote, err := c.SFTP.Open(remotePath)
	if err != nil {
		return fmt.Errorf("sftp open %s: %w", remotePath, err)
	}
	defer remote.Close()

	local, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
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
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("copy %s to %s: %w", src, dst, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", dst, closeErr)
	}
	return nil
}

// DirEntry represents a single file or directory on the remote server.
type DirEntry struct {
	Name    string      // base name
	Path    string      // full remote path
	IsDir   bool        // true if directory
	Size    int64       // file size in bytes
	Mode    os.FileMode // permission bits
	ModTime time.Time   // last modification time
}

// HomeDir returns the remote user's home directory using the SFTP client's
// current working directory (which is the home dir after login).
func (c *Client) HomeDir() (string, error) {
	home, err := c.SFTP.Getwd()
	if err != nil {
		return "", fmt.Errorf("sftp getwd: %w", err)
	}
	return home, nil
}

// ListDir returns the contents of a remote directory, sorted with
// directories first, then files, both alphabetically by name.
func (c *Client) ListDir(remotePath string) ([]DirEntry, error) {
	entries, err := c.SFTP.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("sftp read dir %s: %w", remotePath, err)
	}

	result := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, DirEntry{
			Name:    e.Name(),
			Path:    filepath.Join(remotePath, e.Name()),
			IsDir:   e.IsDir(),
			Size:    e.Size(),
			Mode:    e.Mode(),
			ModTime: e.ModTime(),
		})
	}

	// Sort: directories first, then alphabetically
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir // directories first
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}

// ReadFilePreview reads up to maxBytes from a remote file and returns it as
// a string. If the file appears to be binary, it returns a placeholder message.
func (c *Client) ReadFilePreview(remotePath string, maxBytes int) (string, error) {
	f, err := c.SFTP.Open(remotePath)
	if err != nil {
		return "", fmt.Errorf("sftp open %s: %w", remotePath, err)
	}
	defer f.Close()

	buf := make([]byte, maxBytes)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("sftp read %s: %w", remotePath, err)
	}
	buf = buf[:n]

	// Simple binary detection: check for null bytes
	for _, b := range buf {
		if b == 0 {
			return "[binary file]", nil
		}
	}
	return string(buf), nil
}

// Mkdir creates a remote directory.
func (c *Client) Mkdir(remotePath string) error {
	if err := c.SFTP.Mkdir(remotePath); err != nil {
		return fmt.Errorf("sftp mkdir %s: %w", remotePath, err)
	}
	return nil
}

// Remove removes a remote file.
func (c *Client) Remove(remotePath string) error {
	if err := c.SFTP.Remove(remotePath); err != nil {
		return fmt.Errorf("sftp remove %s: %w", remotePath, err)
	}
	return nil
}

// Rename renames a remote file or directory.
func (c *Client) Rename(oldPath, newPath string) error {
	if err := c.SFTP.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("sftp rename %s -> %s: %w", oldPath, newPath, err)
	}
	return nil
}
