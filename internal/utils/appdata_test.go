package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDir_EnvOverride(t *testing.T) {
	t.Setenv(dataDirEnv, filepath.Join(t.TempDir(), "custom"))
	// 重置 once：本包用 sync.Once，测试需直接调 resolveDataDir
	got, err := resolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(os.Getenv(dataDirEnv))
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveDataDir_ProjectLayout(t *testing.T) {
	t.Setenv(dataDirEnv, "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	got, err := resolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "data")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveDataDir_ExistingDB(t *testing.T) {
	t.Setenv(dataDirEnv, "")
	root := t.TempDir()
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, DBFileName), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	got, err := resolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != data {
		t.Fatalf("got %q want %q", got, data)
	}
}
