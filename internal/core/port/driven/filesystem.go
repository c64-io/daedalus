package driven

import "io/fs"

// FileSystem is a driven port abstracting filesystem side-effects used
// by the core. Keeping it behind an interface lets the core stay free
// of direct os/filepath imports and makes services unit-testable
// without touching disk.
type FileSystem interface {
	// Getwd returns the current working directory.
	Getwd() (string, error)
	// Abs returns an absolute representation of path.
	Abs(path string) (string, error)
	// Stat returns file info for the given path. Implementations must
	// return an error satisfying errors.Is(err, fs.ErrNotExist) when
	// the path does not exist.
	Stat(path string) (fs.FileInfo, error)
	// MkdirAll creates path and any necessary parents with the given perm.
	MkdirAll(path string, perm fs.FileMode) error
	// RemoveAll removes path and any children it contains. It returns nil
	// if the path does not exist.
	RemoveAll(path string) error
	// ReadFile reads the named file and returns its contents.
	ReadFile(path string) ([]byte, error)
	// WriteFile writes data to the named file, creating it if necessary.
	WriteFile(path string, data []byte, perm fs.FileMode) error
}
