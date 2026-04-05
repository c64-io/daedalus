package osfs

import (
	"io/fs"
	"os"
	"path/filepath"
)

// FileSystem is the production adapter for port.FileSystem, backed by
// the host OS via the standard library.
type FileSystem struct{}

// New returns an os-backed FileSystem adapter.
func New() *FileSystem { return &FileSystem{} }

func (FileSystem) Getwd() (string, error)                  { return os.Getwd() }
func (FileSystem) Abs(path string) (string, error)         { return filepath.Abs(path) }
func (FileSystem) Stat(path string) (fs.FileInfo, error)   { return os.Stat(path) }
func (FileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}
