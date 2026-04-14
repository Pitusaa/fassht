package fuzzy_test

import (
	"testing"

	"github.com/juanperetto/fassht/fuzzy"
)

func TestFilter_EmptyQuery_ReturnsAll(t *testing.T) {
	items := []string{"/home/user/file.txt", "/etc/nginx/nginx.conf", "/var/log/app.log"}
	result := fuzzy.Filter(items, "")
	if len(result) != len(items) {
		t.Errorf("expected %d items, got %d", len(items), len(result))
	}
}

func TestFilter_MatchesSubsequence(t *testing.T) {
	items := []string{"/home/user/config.yaml", "/etc/hosts", "/home/user/notes.txt"}
	result := fuzzy.Filter(items, "cfg")
	if len(result) != 1 || result[0] != "/home/user/config.yaml" {
		t.Errorf("expected config.yaml match, got %v", result)
	}
}

func TestFilter_CaseInsensitive(t *testing.T) {
	items := []string{"/home/user/README.md", "/etc/hosts"}
	result := fuzzy.Filter(items, "readme")
	if len(result) != 1 || result[0] != "/home/user/README.md" {
		t.Errorf("expected README.md match, got %v", result)
	}
}

func TestFilter_NoMatch_ReturnsEmpty(t *testing.T) {
	items := []string{"/etc/hosts", "/etc/passwd"}
	result := fuzzy.Filter(items, "zzzzzz")
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %v", result)
	}
}
