// DB Backup Utility (versioned, keep N).

package store

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const defaultKeepBackups = 5

// BackupsDir returns the backups root, creating it on demand.
func BackupsDir() string {
	return filepath.Join(dataDir(), "backups")
}

// BackupInfo describes a captured backup snapshot.
type BackupInfo struct {
	Label     string    `json:"label"`
	Dir       string    `json:"dir"`
	DBPath    string    `json:"dbPath"`
	CreatedAt time.Time `json:"createdAt"`
	SizeBytes int64     `json:"sizeBytes"`
}

// Backup creates a fresh snapshot directory named <label>-<unixSlug> containing
// data.sqlite. After writing, prunes old backups so at most keepN remain. Pass
// keepN<=0 to use the default (5). Safe to call while the app holds the DB
// open — prefers `VACUUM INTO` (SQLite native atomic snapshot) and falls back
// to plain file copy when VACUUM is unavailable.
func Backup(label string, keepN int) (*BackupInfo, error) {
	d, _ := Open() // best-effort; nil ok (fallback to file copy)
	return backupWithConn(d, label, keepN)
}

// backupWithConn is the same as Backup but takes an explicit connection.
// Callers reached from inside Open() (e.g. applyMigrations) MUST use this
// form to avoid re-entering sync.Once and deadlocking.
func backupWithConn(d *sql.DB, label string, keepN int) (*BackupInfo, error) {
	if label == "" {
		label = "manual"
	}
	if keepN <= 0 {
		keepN = defaultKeepBackups
	}
	root := BackupsDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir backups: %w", err)
	}
	slug := fmt.Sprintf("%s-%s", label, time.Now().UTC().Format("20060102T150405Z"))
	dir := filepath.Join(root, slug)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir snap: %w", err)
	}
	dest := filepath.Join(dir, "data.sqlite")

	if err := snapshotIntoWithConn(d, dest); err != nil {
		// Best effort: clean half-written dir then return the error.
		_ = os.RemoveAll(dir)
		return nil, err
	}
	info := &BackupInfo{Label: label, Dir: dir, DBPath: dest, CreatedAt: time.Now().UTC()}
	if st, err := os.Stat(dest); err == nil {
		info.SizeBytes = st.Size()
	}
	_ = pruneBackups(root, keepN)
	return info, nil
}

// ListBackups enumerates existing snapshot directories newest-first.
func ListBackups() ([]BackupInfo, error) {
	root := BackupsDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []BackupInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		full := filepath.Join(root, e.Name())
		dbp := filepath.Join(full, "data.sqlite")
		st, err := os.Stat(dbp)
		if err != nil {
			continue
		}
		info, _ := e.Info()
		ct := st.ModTime().UTC()
		if info != nil {
			ct = info.ModTime().UTC()
		}
		out = append(out, BackupInfo{
			Label:     e.Name(),
			Dir:       full,
			DBPath:    dbp,
			CreatedAt: ct,
			SizeBytes: st.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// snapshotIntoWithConn produces a consistent copy of the live DB at dest.
// Tries `VACUUM INTO '<dest>'` first (SQLite native, atomic, WAL-aware);
// falls back to a plain file-copy of DBPath() when the connection is nil
// (caller invoked before Open() finished, e.g. pre-migrate path).
func snapshotIntoWithConn(d *sql.DB, dest string) error {
	if d != nil {
		// SQLite ≥3.27 supports VACUUM INTO. modernc.org/sqlite ships a recent
		// build, so this is the preferred path.
		if _, err := d.Exec(`VACUUM INTO ?`, dest); err == nil {
			return nil
		}
		// fall through to copy
	}
	src := DBPath()
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

// pruneBackups deletes oldest snapshots so at most keepN remain.
func pruneBackups(root string, keepN int) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	type item struct {
		name  string
		mtime time.Time
	}
	var dirs []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		dirs = append(dirs, item{name: e.Name(), mtime: info.ModTime()})
	}
	if len(dirs) <= keepN {
		return nil
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].mtime.After(dirs[j].mtime) })
	for _, d := range dirs[keepN:] {
		_ = os.RemoveAll(filepath.Join(root, d.name))
	}
	return nil
}
