// Package fake is the local stand-in of the payouts' private document storage (P2): it writes
// each object as a file under a root directory, so attachments survive an API restart in
// compose. Production replaces it with an object-storage adapter behind the same port.
package fake

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/erfajc97/lenapay/apps/api/internal/payouts"
)

// DiskDocumentStore stores objects under Root (keys are slash-separated relative paths).
type DiskDocumentStore struct {
	Root string
}

var _ payouts.DocumentStore = (*DiskDocumentStore)(nil)

// NewDiskDocumentStore returns a store rooted at dir ("" = <os temp>/lenapay-documents).
func NewDiskDocumentStore(dir string) *DiskDocumentStore {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "lenapay-documents")
	}
	return &DiskDocumentStore{Root: dir}
}

// Put implements payouts.DocumentStore. Keys are built by the service from ids only; a key
// that would leave Root is refused anyway.
func (s *DiskDocumentStore) Put(_ context.Context, key, _ string, content io.Reader) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("documents: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // path is checked to stay under Root
	if err != nil {
		return fmt.Errorf("documents: create: %w", err)
	}
	if _, err := io.Copy(f, content); err != nil {
		_ = f.Close()
		return fmt.Errorf("documents: write: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("documents: close: %w", err)
	}
	return nil
}

// Get returns a stored object (tests and local inspection).
func (s *DiskDocumentStore) Get(key string) ([]byte, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path) //nolint:gosec // path is checked to stay under Root
}

func (s *DiskDocumentStore) path(key string) (string, error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", fmt.Errorf("documents: root: %w", err)
	}
	path := filepath.Join(root, filepath.FromSlash(key))
	if key == "" || !strings.HasPrefix(path, root+string(filepath.Separator)) {
		return "", fmt.Errorf("documents: key %q leaves the root", key)
	}
	return path, nil
}
