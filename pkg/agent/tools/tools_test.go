package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitIgnoreMatcherSubdirectoryAndMutation(t *testing.T) {
	// Create a temp directory hierarchy
	tmp, err := os.MkdirTemp("", "rho-test-gitignore-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmp)

	// Create structure:
	// tmp/
	//   .gitignore (contains "ignored-at-root.txt")
	//   subdir/
	//     .gitignore (contains "ignored-in-subdir/")
	//     file.txt
	//     ignored-at-root.txt
	//     ignored-in-subdir/
	//       secret.txt

	err = os.WriteFile(filepath.Join(tmp, ".gitignore"), []byte("ignored-at-root.txt\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write root .gitignore: %v", err)
	}

	subdir := filepath.Join(tmp, "subdir")
	err = os.MkdirAll(filepath.Join(subdir, "ignored-in-subdir"), 0755)
	if err != nil {
		t.Fatalf("failed to create subdirs: %v", err)
	}

	err = os.WriteFile(filepath.Join(subdir, ".gitignore"), []byte("ignored-in-subdir/\n"), 0644)
	if err != nil {
		t.Fatalf("failed to write subdir .gitignore: %v", err)
	}

	err = os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = os.WriteFile(filepath.Join(subdir, "ignored-at-root.txt"), []byte("ignored"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err = os.WriteFile(filepath.Join(subdir, "ignored-in-subdir", "secret.txt"), []byte("secret"), 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// 1. Verify newGitIgnoreMatcher doesn't mutate or break root path:
	m := newGitIgnoreMatcher(subdir)
	if m == nil {
		t.Fatalf("newGitIgnoreMatcher returned nil")
	}

	// 2. Walk directory with walkGitIgnore and record visited paths
	visited := make(map[string]bool)
	err = walkGitIgnore(tmp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(tmp, path)
		if err == nil {
			visited[rel] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walkGitIgnore failed: %v", err)
	}

	// Expected to visit:
	// "."
	// "subdir"
	// "subdir/file.txt"
	// ".gitignore"
	// "subdir/.gitignore"
	// (Note: "subdir/ignored-at-root.txt" and "subdir/ignored-in-subdir" should NOT be visited)

	if visited["subdir/ignored-at-root.txt"] {
		t.Errorf("expected subdir/ignored-at-root.txt to be ignored")
	}
	if visited["subdir/ignored-in-subdir"] {
		t.Errorf("expected subdir/ignored-in-subdir directory to be ignored")
	}
	if visited["subdir/ignored-in-subdir/secret.txt"] {
		t.Errorf("expected files in ignored directory to be ignored")
	}
	if !visited["subdir/file.txt"] {
		t.Errorf("expected subdir/file.txt to be visited")
	}
}

func TestEditDiffFuzzyMappingPreservation(t *testing.T) {
	// Original content has smart quotes and trailing spaces outside the edit zone:
	originalContent := "Hello \u201CWorld\u201D   \nTarget line here\nSome other text \u2014 trailing space   \n"
	
	edits := []editDef{
		{
			oldText: "Some other text - trailing space", // uses ASCII hyphen instead of em-dash -> triggers fuzzy matching
			newText: "New third line",
		},
	}
	
	result, err := applyEditsToNormalizedContent(originalContent, edits, "dummy_path.txt")
	if err != nil {
		t.Fatalf("applyEditsToNormalizedContent failed: %v", err)
	}
	
	// The output should preserve the smart quotes and trailing spaces in the first line, but apply the edit on the third line!
	expectedContent := "Hello \u201CWorld\u201D   \nTarget line here\nNew third line\n"
	if result.newContent != expectedContent {
		t.Errorf("expected content:\n%q\ngot:\n%q", expectedContent, result.newContent)
	}
	
	if result.baseContent != originalContent {
		t.Errorf("expected baseContent to be exactly the original content, got %q", result.baseContent)
	}
}
