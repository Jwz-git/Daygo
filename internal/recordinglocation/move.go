// Package recordinglocation moves recorded media while keeping the database's
// relative segment paths valid. It never opens the business database itself.
package recordinglocation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jwz-git/Daygo/internal/settings"
)

const (
	DirectoryKey      = settings.KeyStorageRecordingsDirectory
	MigrationKey      = "storage.recordingsMigration"
	RecoveryGuardFile = "recordings-location.guard"
)

type settingsRepo interface {
	Get(context.Context, string) (string, bool, error)
	SetMany(context.Context, map[string]string) error
	Delete(context.Context, string) error
}

type migration struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Phase  string `json:"phase"`
}

// Active resolves the effective root. An absent key retains the original
// application support layout. A malformed stored value fails closed.
func Active(ctx context.Context, repo settingsRepo, defaultRoot string) (string, error) {
	raw, ok, err := repo.Get(ctx, DirectoryKey)
	if err != nil {
		return "", err
	}
	if !ok {
		return defaultRoot, nil
	}
	var root string
	if err := json.Unmarshal([]byte(raw), &root); err != nil || !filepath.IsAbs(root) {
		return "", errors.New("recording location: invalid stored directory")
	}
	return filepath.Clean(root), nil
}

// Pending returns an interrupted migration. Before the commit, Source remains
// authoritative. After it, Target remains authoritative even if cleanup fails.
func Pending(ctx context.Context, repo settingsRepo) (string, string, string, error) {
	raw, ok, err := repo.Get(ctx, MigrationKey)
	if err != nil || !ok {
		return "", "", "", err
	}
	var m migration
	if err := json.Unmarshal([]byte(raw), &m); err != nil || !filepath.IsAbs(m.Source) || !filepath.IsAbs(m.Target) || (m.Phase != "copying" && m.Phase != "committed") {
		return "", "", "", errors.New("recording location: invalid migration state")
	}
	return m.Source, m.Target, m.Phase, nil
}

func storeMigration(ctx context.Context, repo settingsRepo, m migration) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return repo.SetMany(ctx, map[string]string{MigrationKey: string(b)})
}

// Move copies and verifies all regular files before atomically switching the
// setting. It leaves the source intact until the setting has committed. On an
// interrupted copy, retrying the same target is safe and resumes file by file.
func Move(ctx context.Context, repo settingsRepo, defaultRoot, target string) error {
	source, err := Active(ctx, repo, defaultRoot)
	if err != nil {
		return err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if source == target {
		return nil
	}
	if info, err := os.Stat(source); err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil && !info.IsDir() {
		return errors.New("recording location: source is not a directory")
	}
	if err := validatePair(source, target); err != nil {
		return err
	}
	oldSource, oldTarget, phase, err := Pending(ctx, repo)
	if err != nil {
		return err
	}
	if phase != "" && (oldSource != source || oldTarget != target || phase != "copying") {
		return errors.New("recording location: another migration needs recovery")
	}
	if phase == "" {
		if err := validateTarget(target); err != nil {
			return err
		}
		if err := storeMigration(ctx, repo, migration{source, target, "copying"}); err != nil {
			return err
		}
	}
	if err := copyTree(ctx, source, target); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// A database backup taken before this move still names Source. Once Source
	// is cleaned up, restoring that backup automatically would point every
	// screenshot at missing media. The guard makes storage refuse such backups.
	guard := filepath.Join(filepath.Dir(defaultRoot), RecoveryGuardFile)
	if err := writeGuard(guard, target); err != nil {
		return err
	}
	rootJSON, _ := json.Marshal(target)
	committed, _ := json.Marshal(migration{source, target, "committed"})
	// Cancellation must not interrupt the atomic settings commit after the copy
	// has been verified; otherwise the caller could mistake a committed move
	// for a canceled one and restart capture at Source.
	if err := repo.SetMany(context.Background(), map[string]string{DirectoryKey: string(rootJSON), MigrationKey: string(committed)}); err != nil {
		return err
	}
	return nil
}

func writeGuard(path, target string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(target); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Finish removes only source files whose target copy still matches. Cleanup is
// repeatable and may be deferred after a crash. A missing target is an error,
// never a reason to delete the source.
func Finish(ctx context.Context, repo settingsRepo) error {
	source, target, phase, err := Pending(ctx, repo)
	if err != nil || phase != "committed" {
		return err
	}
	var directories []string
	if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != source {
				directories = append(directories, path)
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return errors.New("recording location: source contains a link or special file")
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetFile := filepath.Join(target, rel)
		if err := requirePlainParents(target, targetFile); err != nil {
			return err
		}
		match, err := sameFile(ctx, path, targetFile)
		if err != nil || !match {
			return fmt.Errorf("recording location: target verification failed: %w", err)
		}
		return os.Remove(path)
	}); err != nil {
		return err
	}
	for i := len(directories) - 1; i >= 0; i-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := os.Remove(directories[i]); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	// Keep the old root itself. It can be selected again for a later move.
	return repo.Delete(ctx, MigrationKey)
}

func requirePlainParents(root, path string) error {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		info, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("recording location: target directory is a link or special file: %s", dir)
		}
		if dir == root {
			return nil
		}
		if filepath.Dir(dir) == dir {
			return errors.New("recording location: target path escaped recording root")
		}
	}
}

func validatePair(source, target string) error {
	if !filepath.IsAbs(target) || target == filepath.VolumeName(target)+string(filepath.Separator) {
		return errors.New("recording location: choose a directory below a volume root")
	}
	a, err := filepath.EvalSymlinks(source)
	if err == nil {
		source = a
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err == nil {
		target = filepath.Join(parent, filepath.Base(target))
	}
	rel, err := filepath.Rel(source, target)
	if err == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")) {
		return errors.New("recording location: target is inside source")
	}
	rel, err = filepath.Rel(target, source)
	if err == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")) {
		return errors.New("recording location: source is inside target")
	}
	return nil
}

func validateTarget(target string) error {
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("recording location: target must be a regular directory")
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("recording location: target directory must be empty")
	}
	return nil
}

func copyTree(ctx context.Context, source, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if os.IsNotExist(walkErr) && path == source {
			return ensureDirectory(target)
		}
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		to := filepath.Join(target, rel)
		if entry.IsDir() {
			return ensureDirectory(to)
		}
		if !entry.Type().IsRegular() {
			return errors.New("recording location: source contains a link or special file")
		}
		if err := ensureRegularIfExists(to); err != nil {
			return err
		}
		if match, err := sameFile(ctx, path, to); err == nil && match {
			return nil
		}
		return copyFile(ctx, path, to)
	})
}

func ensureDirectory(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return os.Mkdir(path, 0700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("recording location: target directory is a link or special file: %s", path)
	}
	return nil
}

func ensureRegularIfExists(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("recording location: target file is a link or special file: %s", path)
	}
	return nil
}

func copyFile(ctx context.Context, source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(target), ".daygo-moving-*")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	if _, err = io.Copy(out, &contextReader{ctx, in}); err != nil {
		out.Close()
		return err
	}
	if err = out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	// A retry may have left an earlier copy. Windows Rename cannot replace it.
	if err := ensureRegularIfExists(target); err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = os.Rename(tmp, target); err != nil {
		return err
	}
	match, err := sameFile(ctx, source, target)
	if err != nil {
		return err
	}
	if !match {
		return errors.New("recording location: copied file differs")
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func sameFile(ctx context.Context, a, b string) (bool, error) {
	if err := ensureRegularIfExists(a); err != nil {
		return false, err
	}
	if err := ensureRegularIfExists(b); err != nil {
		return false, err
	}
	x, err := os.Open(a)
	if err != nil {
		return false, err
	}
	defer x.Close()
	y, err := os.Open(b)
	if err != nil {
		return false, err
	}
	defer y.Close()
	xInfo, err := x.Stat()
	if err != nil {
		return false, err
	}
	yInfo, err := y.Stat()
	if err != nil {
		return false, err
	}
	if xInfo.Size() != yInfo.Size() {
		return false, nil
	}
	h1, h2 := sha256.New(), sha256.New()
	if _, err := io.Copy(h1, &contextReader{ctx, x}); err != nil {
		return false, err
	}
	if _, err := io.Copy(h2, &contextReader{ctx, y}); err != nil {
		return false, err
	}
	return string(h1.Sum(nil)) == string(h2.Sum(nil)), nil
}
