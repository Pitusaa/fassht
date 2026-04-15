package ssh_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	fasshtssh "github.com/Pitusaa/fassht/ssh"
)

func TestTempFilePath_ContainsFilename(t *testing.T) {
	path := fasshtssh.TempFilePath("/var/www/config.yaml")
	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("expected path to end with config.yaml, got %s", path)
	}
	if !strings.Contains(path, "fassht_") {
		t.Errorf("expected path to contain 'fassht_', got %s", path)
	}
}

func TestTempFilePath_DifferentRemotesProduceDifferentPaths(t *testing.T) {
	p1 := fasshtssh.TempFilePath("/var/www/a.txt")
	p2 := fasshtssh.TempFilePath("/home/user/a.txt")
	if p1 == p2 {
		t.Error("different remote paths should produce different temp paths")
	}
}

func TestUploadDownloadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "dest.txt")
	content := []byte("hello fassht")

	os.WriteFile(src, content, 0644)

	if err := fasshtssh.CopyFile(src, dst); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("expected %q, got %q", content, got)
	}
}
