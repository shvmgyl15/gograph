package rootfind

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot_NoGographDir_FallsBackToCwd(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got := FindRoot()
	if got != "." {
		t.Errorf("FindRoot() = %q, want %q (no .gograph anywhere)", got, ".")
	}
}

func TestFindRoot_FromRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".gograph"), 0o755); err != nil {
		t.Fatalf("mkdir .gograph: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, _ := filepath.EvalSymlinks(FindRoot())
	want, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Errorf("FindRoot() = %q, want %q", got, want)
	}
}

func TestFindRoot_FromSubdirectory(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "internal", "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".gograph"), 0o755); err != nil {
		t.Fatalf("mkdir .gograph: %v", err)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	got, _ := filepath.EvalSymlinks(FindRoot())
	want, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Errorf("FindRoot() from subdirectory = %q, want %q", got, want)
	}
}
