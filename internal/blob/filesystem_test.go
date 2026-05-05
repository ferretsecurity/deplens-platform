package blob

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSystemStorePutPersistsBytesAtArtifactKey(t *testing.T) {
	root := t.TempDir()
	store := NewFileSystemStore(root)

	key, sha, err := store.Put(context.Background(), []byte(`{"root":".","manifests":[]}`))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if key == "" {
		t.Fatal("Put() key = empty, want non-empty")
	}
	if sha == "" {
		t.Fatal("Put() sha = empty, want non-empty")
	}

	if _, err := os.Stat(filepath.Join(root, key)); err != nil {
		t.Fatalf("stored artifact missing: %v", err)
	}
}
