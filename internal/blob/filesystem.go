package blob

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

type FileSystemStore struct {
	root string
}

func NewFileSystemStore(root string) *FileSystemStore {
	return &FileSystemStore{root: root}
}

func (s *FileSystemStore) Put(_ context.Context, payload []byte) (string, string, error) {
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	key := filepath.Join(hash[:2], hash[2:4], hash+".json")
	fullPath := filepath.Join(s.root, key)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(fullPath, payload, 0o644); err != nil {
		return "", "", err
	}

	return key, hash, nil
}
