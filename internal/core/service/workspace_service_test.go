package service_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/c64-io/daedalus/internal/core/domain"
	"github.com/c64-io/daedalus/internal/core/port"
	"github.com/c64-io/daedalus/internal/core/service"
)

// fakeFS is an in-memory FileSystem fake sufficient for WorkspaceService
// tests. It tracks created directories and pre-existing paths.
type fakeFS struct {
	cwd      string
	existing map[string]bool // paths that should report as present to Stat
	created  map[string]fs.FileMode
	getwdErr error
	absErr   error
	mkdirErr error
}

func newFakeFS(cwd string) *fakeFS {
	return &fakeFS{
		cwd:      cwd,
		existing: map[string]bool{},
		created:  map[string]fs.FileMode{},
	}
}

func (f *fakeFS) Getwd() (string, error) {
	if f.getwdErr != nil {
		return "", f.getwdErr
	}
	return f.cwd, nil
}

func (f *fakeFS) Abs(path string) (string, error) {
	if f.absErr != nil {
		return "", f.absErr
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(f.cwd, path), nil
}

func (f *fakeFS) Stat(path string) (fs.FileInfo, error) {
	if f.existing[path] {
		return fakeFileInfo{name: filepath.Base(path)}, nil
	}
	return nil, fs.ErrNotExist
}

func (f *fakeFS) MkdirAll(path string, perm fs.FileMode) error {
	if f.mkdirErr != nil {
		return f.mkdirErr
	}
	f.created[path] = perm
	return nil
}

type fakeFileInfo struct{ name string }

func (f fakeFileInfo) Name() string     { return f.name }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return true }
func (fakeFileInfo) Sys() any           { return nil }

// fakeRepo captures the arguments CreateDatabase is called with and
// answers ReadMetadata with a pre-seeded value. Tests that exercise
// the read path set readMeta/readErr before invoking Status.
type fakeRepo struct {
	// CreateDatabase captures.
	createCalled bool
	dbDir        string
	meta         domain.WorkspaceMetadata
	createErr    error

	// ReadMetadata seeds.
	readMeta domain.WorkspaceMetadata
	readErr  error
}

func (r *fakeRepo) CreateDatabase(_ context.Context, dbDir string, meta domain.WorkspaceMetadata) error {
	r.createCalled = true
	r.dbDir = dbDir
	r.meta = meta
	return r.createErr
}

func (r *fakeRepo) ReadMetadata(_ context.Context, _ string) (domain.WorkspaceMetadata, error) {
	if r.readErr != nil {
		return domain.WorkspaceMetadata{}, r.readErr
	}
	return r.readMeta, nil
}

func TestInit_Success_Go(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/home/alice/proj")
	repo := &fakeRepo{}
	svc := service.NewWorkspaceService(fs, repo)

	ws, err := svc.Init(context.Background(), port.InitRequest{
		Target: domain.TargetGo,
	})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	wantDir := "/home/alice/proj/d7"
	wantDB := "/home/alice/proj/d7/.db"
	if ws.Dir != wantDir {
		t.Errorf("ws.Dir = %q, want %q", ws.Dir, wantDir)
	}
	if ws.DBDir != wantDB {
		t.Errorf("ws.DBDir = %q, want %q", ws.DBDir, wantDB)
	}
	if _, ok := fs.created[wantDB]; !ok {
		t.Errorf("expected MkdirAll(%q); created=%v", wantDB, fs.created)
	}
	if !repo.createCalled {
		t.Fatal("expected CreateDatabase to be called")
	}
	if repo.dbDir != wantDB {
		t.Errorf("repo.dbDir = %q, want %q", repo.dbDir, wantDB)
	}
	if repo.meta.Target != domain.TargetGo {
		t.Errorf("repo.meta.Target = %q, want %q", repo.meta.Target, domain.TargetGo)
	}
}

func TestInit_Success_TypeScript_ExplicitRoot(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/anywhere")
	repo := &fakeRepo{}
	svc := service.NewWorkspaceService(fs, repo)

	ws, err := svc.Init(context.Background(), port.InitRequest{
		RootDir: "/srv/app",
		Target:  domain.TargetTypeScript,
	})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}
	if ws.RootDir != "/srv/app" {
		t.Errorf("ws.RootDir = %q, want %q", ws.RootDir, "/srv/app")
	}
	if repo.meta.Target != domain.TargetTypeScript {
		t.Errorf("repo.meta.Target = %q, want %q", repo.meta.Target, domain.TargetTypeScript)
	}
}

func TestInit_NoTarget(t *testing.T) {
	t.Parallel()

	svc := service.NewWorkspaceService(newFakeFS("/x"), &fakeRepo{})
	_, err := svc.Init(context.Background(), port.InitRequest{RootDir: "/x"})
	if !errors.Is(err, service.ErrNoTarget) {
		t.Fatalf("Init err = %v, want ErrNoTarget", err)
	}
}

func TestInit_WorkspaceAlreadyExists(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/home/alice/proj")
	fs.existing["/home/alice/proj/d7"] = true
	repo := &fakeRepo{}
	svc := service.NewWorkspaceService(fs, repo)

	_, err := svc.Init(context.Background(), port.InitRequest{
		Target: domain.TargetGo,
	})
	if !errors.Is(err, service.ErrWorkspaceExists) {
		t.Fatalf("Init err = %v, want ErrWorkspaceExists", err)
	}
	if repo.createCalled {
		t.Error("CreateDatabase should not be called when workspace exists")
	}
}

func TestStatus_Success(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/home/alice/proj")
	fs.existing["/home/alice/proj/d7/.db"] = true
	repo := &fakeRepo{
		readMeta: domain.WorkspaceMetadata{Target: domain.TargetGo},
	}
	svc := service.NewWorkspaceService(fs, repo)

	st, err := svc.Status(context.Background(), "")
	if err != nil {
		t.Fatalf("Status returned unexpected error: %v", err)
	}
	if st.Workspace.Dir != "/home/alice/proj/d7" {
		t.Errorf("st.Workspace.Dir = %q, want %q", st.Workspace.Dir, "/home/alice/proj/d7")
	}
	if st.Metadata.Target != domain.TargetGo {
		t.Errorf("st.Metadata.Target = %q, want %q", st.Metadata.Target, domain.TargetGo)
	}
}

func TestStatus_ExplicitRoot(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/anywhere")
	fs.existing["/srv/app/d7/.db"] = true
	repo := &fakeRepo{
		readMeta: domain.WorkspaceMetadata{Target: domain.TargetTypeScript},
	}
	svc := service.NewWorkspaceService(fs, repo)

	st, err := svc.Status(context.Background(), "/srv/app")
	if err != nil {
		t.Fatalf("Status returned unexpected error: %v", err)
	}
	if st.Metadata.Target != domain.TargetTypeScript {
		t.Errorf("st.Metadata.Target = %q, want %q", st.Metadata.Target, domain.TargetTypeScript)
	}
}

func TestStatus_WorkspaceNotFound(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/empty")
	svc := service.NewWorkspaceService(fs, &fakeRepo{})

	_, err := svc.Status(context.Background(), "/empty")
	if !errors.Is(err, service.ErrWorkspaceNotFound) {
		t.Fatalf("Status err = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestStatus_ReadMetadataError(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/home/alice/proj")
	fs.existing["/home/alice/proj/d7/.db"] = true
	repo := &fakeRepo{readErr: errors.New("boom")}
	svc := service.NewWorkspaceService(fs, repo)

	_, err := svc.Status(context.Background(), "")
	if err == nil {
		t.Fatal("expected Status to return error when repo.ReadMetadata fails")
	}
}
