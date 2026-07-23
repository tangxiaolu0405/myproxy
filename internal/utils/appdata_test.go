package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveDataDir_EnvOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom")
	t.Setenv(dataDirEnv, custom)
	got, err := resolveDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(custom) {
		t.Fatalf("got %q want %q", got, custom)
	}
}

func TestFindAppBundleRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path separators differ")
	}
	exe := "/Users/me/Apps/LProxy.app/Contents/MacOS/LProxy"
	got := findAppBundleRoot(exe)
	want := "/Users/me/Apps/LProxy.app"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if findAppBundleRoot("/opt/LProxy") != "" {
		t.Fatal("expected empty for non-bundle")
	}
}

func TestResolveDataDir_BesideExecutable(t *testing.T) {
	t.Setenv(dataDirEnv, "")
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// launchDir 依赖真实 Executable，这里直接测 findAppBundleRoot + 旁路规则
	app := filepath.Join(root, "LProxy.app", "Contents", "MacOS")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(app, "LProxy")
	bundle := findAppBundleRoot(exe)
	if bundle != filepath.Join(root, "LProxy.app") {
		t.Fatalf("bundle=%q", bundle)
	}
	launch := filepath.Dir(bundle)
	got := filepath.Join(launch, "data")
	want := filepath.Join(root, "data")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
