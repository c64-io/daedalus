package domain

import "path/filepath"

// Workspace represents an initialized d7 workspace on disk.
type Workspace struct {
	RootDir string // directory in which `d7 init` was run
	Dir     string // <RootDir>/d7
	DBDir   string // <RootDir>/d7/.db
}

// NewWorkspace builds a Workspace descriptor rooted at rootDir.
func NewWorkspace(rootDir string) *Workspace {
	dir := filepath.Join(rootDir, "d7")
	return &Workspace{
		RootDir: rootDir,
		Dir:     dir,
		DBDir:   filepath.Join(dir, ".db"),
	}
}
