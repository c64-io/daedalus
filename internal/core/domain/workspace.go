package domain

import "path/filepath"

// Workspace represents an initialized d7 workspace on disk. It is a
// pure path descriptor; the configuration associated with a workspace
// lives in WorkspaceMetadata, which is persisted in the workspace's
// database.
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

// WorkspaceMetadata is the configuration written into a workspace's
// database at creation time. It is immutable for the life of the
// workspace in v1 — there is no service-layer operation to mutate it.
type WorkspaceMetadata struct {
	// Targets is the set of code-generation and verification targets
	// declared at `d7 init`. At least one target is required. The
	// slice is canonically ordered (see ParseTargets).
	Targets []Target
}
