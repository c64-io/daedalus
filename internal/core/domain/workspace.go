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

// ProjectDescriptionTemplate is the default content written to
// project.md when a workspace is initialized. AI-assisted commands use
// this file as context; the user is expected to replace the template
// with a real product description.
const ProjectDescriptionTemplate = `# Project Description

<!-- Edit this file to describe your product. AI-assisted commands
     (suggest, expand, refine, generate) use it as context. -->

## What is this product?

## Who is it for?

## Key capabilities

## Tech stack

## Constraints & non-goals
`

// ProjectDescription holds the workspace's project description
// content along with metadata about whether it has been customized.
type ProjectDescription struct {
	Content   string
	IsDefault bool // true when content matches the template exactly
}

// WorkspaceMetadata is the configuration written into a workspace's
// database at creation time. It is immutable for the life of the
// workspace in v1 — there is no service-layer operation to mutate it.
type WorkspaceMetadata struct {
	// Target is the single code-generation and verification target
	// declared at `d7 init`. v1 workspaces are single-target;
	// monorepo / multi-target support is a v2 ambition.
	Target Target
}

// WorkspaceStatus is a read-only snapshot of an existing workspace,
// combining its on-disk location with the metadata persisted in its
// database. It is the return value of the WorkspaceStatusReader use
// case.
type WorkspaceStatus struct {
	Workspace          *Workspace
	Metadata           WorkspaceMetadata
	ProjectDescription *ProjectDescription
}
