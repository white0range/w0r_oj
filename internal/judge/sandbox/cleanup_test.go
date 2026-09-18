package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"gojo/config"
)

func TestSandboxLabels(t *testing.T) {
	previousEnv := config.GlobalConfig.App.Env
	config.GlobalConfig.App.Env = "test"
	t.Cleanup(func() { config.GlobalConfig.App.Env = previousEnv })

	labels := sandboxLabels("runtime", 42)

	if labels[sandboxLabelKey] != "true" {
		t.Fatalf("sandbox label = %q, want true", labels[sandboxLabelKey])
	}
	if labels[sandboxTypeLabelKey] != "runtime" {
		t.Fatalf("sandbox type label = %q, want runtime", labels[sandboxTypeLabelKey])
	}
	if labels[sandboxSubmissionLabelKey] != "42" {
		t.Fatalf("submission label = %q, want 42", labels[sandboxSubmissionLabelKey])
	}
	if labels[sandboxOwnerLabelKey] != "test" {
		t.Fatalf("owner label = %q, want test", labels[sandboxOwnerLabelKey])
	}
}

func TestCleanupOrphanedWorkDirsOnlyRemovesJudgeDirectories(t *testing.T) {
	root := t.TempDir()
	judgeDir := filepath.Join(root, "judge_123")
	keepDir := filepath.Join(root, "uploads")
	judgeNamedFile := filepath.Join(root, "judge_keep.txt")

	if err := os.Mkdir(judgeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(judgeDir, "solution"), []byte("binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(keepDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(judgeNamedFile, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := cleanupOrphanedWorkDirs(root)
	if err != nil {
		t.Fatalf("cleanupOrphanedWorkDirs() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, err := os.Stat(judgeDir); !os.IsNotExist(err) {
		t.Fatalf("judge directory still exists or stat failed unexpectedly: %v", err)
	}
	if _, err := os.Stat(keepDir); err != nil {
		t.Fatalf("non-judge directory was removed: %v", err)
	}
	if _, err := os.Stat(judgeNamedFile); err != nil {
		t.Fatalf("judge-prefixed regular file was removed: %v", err)
	}
}

func TestCleanupOrphanedWorkDirsAllowsMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")
	removed, err := cleanupOrphanedWorkDirs(root)
	if err != nil {
		t.Fatalf("cleanupOrphanedWorkDirs() error = %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
}
