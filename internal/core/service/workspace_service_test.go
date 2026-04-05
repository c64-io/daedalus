package service_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
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

func (f fakeFileInfo) Name() string       { return f.name }
func (fakeFileInfo) Size() int64          { return 0 }
func (fakeFileInfo) Mode() fs.FileMode    { return fs.ModeDir }
func (fakeFileInfo) ModTime() time.Time   { return time.Time{} }
func (fakeFileInfo) IsDir() bool          { return true }
func (fakeFileInfo) Sys() any             { return nil }

// fakeRepo captures the arguments CreateDatabase is called with so
// tests can assert on them.
type fakeRepo struct {
	called  bool
	dbDir   string
	meta    domain.WorkspaceMetadata
	createErr error
}

func (r *fakeRepo) CreateDatabase(_ context.Context, dbDir string, meta domain.WorkspaceMetadata) error {
	r.called = true
	r.dbDir = dbDir
	r.meta = meta
	return r.createErr
}

func TestInit_Success_SingleTarget(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/home/alice/proj")
	repo := &fakeRepo{}
	svc := service.NewWorkspaceService(fs, repo)

	ws, err := svc.Init(context.Background(), port.InitRequest{
		RootDir: "",
		Targets: []domain.Target{domain.TargetGo},
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
		t.Errorf("expected MkdirAll(%q) to have been called; created=%v", wantDB, fs.created)
	}
	if !repo.called {
		t.Fatal("expected CreateDatabase to be called")
	}
	if repo.dbDir != wantDB {
		t.Errorf("repo.dbDir = %q, want %q", repo.dbDir, wantDB)
	}
	if !reflect.DeepEqual(repo.meta.Targets, []domain.Target{domain.TargetGo}) {
		t.Errorf("repo.meta.Targets = %v, want [go]", repo.meta.Targets)
	}
}

func TestInit_Success_MultiTarget_ExplicitRoot(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/anywhere")
	repo := &fakeRepo{}
	svc := service.NewWorkspaceService(fs, repo)

	ws, err := svc.Init(context.Background(), port.InitRequest{
		RootDir: "/srv/app",
		Targets: []domain.Target{domain.TargetGo, domain.TargetTypeScript},
	})
	if err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}
	if ws.RootDir != "/srv/app" {
		t.Errorf("ws.RootDir = %q, want %q", ws.RootDir, "/srv/app")
	}
	want := []domain.Target{domain.TargetGo, domain.TargetTypeScript}
	if !reflect.DeepEqual(repo.meta.Targets, want) {
		t.Errorf("repo.meta.Targets = %v, want %v", repo.meta.Targets, want)
	}
}

func TestInit_NoTargets(t *testing.T) {
	t.Parallel()

	svc := service.NewWorkspaceService(newFakeFS("/x"), &fakeRepo{})
	_, err := svc.Init(context.Background(), port.InitRequest{RootDir: "/x"})
	if !errors.Is(err, service.ErrNoTargets) {
		t.Fatalf("Init err = %v, want ErrNoTargets", err)
	}
}

func TestInit_WorkspaceAlreadyExists(t *testing.T) {
	t.Parallel()

	fs := newFakeFS("/home/alice/proj")
	fs.existing["/home/alice/proj/d7"] = true
	repo := &fakeRepo{}
	svc := service.NewWorkspaceService(fs, repo)

	_, err := svc.Init(context.Background(), port.InitRequest{
		Targets: []domain.Target{domain.TargetGo},
	})
	if !errors.Is(err, service.ErrWorkspaceExists) {
		t.Fatalf("Init err = %v, want ErrWorkspaceExists", err)
	}
	if repo.called {
		t.Error("CreateDatabase should not be called when workspace exists")
	}
}
