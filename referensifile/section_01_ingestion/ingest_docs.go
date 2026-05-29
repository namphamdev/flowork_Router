package ingestor

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/teetah2402/flowork/internal/fsutil"
)

// IngestDocuments memindai semua subfolder di docs/ untuk dimasukkan ke tabel
// documents (atau immune_system khusus bug). Setelah ingest selesai, isi
// folder docs/ bisa dihapus karena DB adalah sumber kebenaran.
//
// Mapping filesystem → DB:
//
//	docs/plan/*.md       → documents type='roadmap'
//	docs/tutorial/*.md   → documents type='tutorial'
//	docs/bug/*.md        → immune_system type='bug_doc'
//	docs/archive/*.md    → documents type='archive'
//	docs/arsenal/*.md    → documents type='arsenal'
//	docs/brain/*.md      → documents type='brain-doc'
//	docs/changelog/*.md  → documents type='changelog'
//	docs/changelog.md    → documents type='changelog'
//	docs/conventions/*.md→ documents type='convention'
//	docs/inventory/*.md  → documents type='inventory'
//	docs/inventory.md    → documents type='inventory'
//	docs/*.md (root)     → documents type='general'
func IngestDocuments(db *sql.DB, projectRoot string) (int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx documents: %w", err)
	}
	defer tx.Rollback()

	count := 0
	docsRoot := filepath.Join(projectRoot, "docs")

	// Folder subdir → DB type mapping (selain bug yang ke immune_system).
	folderToType := map[string]string{
		"plan":        "roadmap",
		"tutorial":    "tutorial",
		"archive":     "archive",
		"arsenal":     "arsenal",
		"brain":       "brain-doc",
		"changelog":   "changelog",
		"conventions": "convention",
		"inventory":   "inventory",
	}

	for folder, docType := range folderToType {
		root := filepath.Join(docsRoot, folder)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			relPath, _ := filepath.Rel(root, path)
			title := filepath.ToSlash(relPath)
			content, err := fsutil.SafeReadFile(path)
			if err != nil {
				return nil
			}
			author := "system"
			if strings.HasPrefix(title, "warga/") {
				parts := strings.Split(title, "/")
				if len(parts) >= 2 {
					author = parts[1]
				}
			}
			_, err = tx.Exec(`INSERT INTO documents (type, title, content, author)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(type, title) DO UPDATE SET
					content=excluded.content,
					updated_at=CURRENT_TIMESTAMP`,
				docType, title, string(content), author)
			if err == nil {
				count++
			} else {
				log.Printf("fq-brain: error inserting %s/%s: %v", docType, title, err)
			}
			return nil
		})
		if err != nil {
			log.Printf("fq-brain: walk %s error: %v", root, err)
		}
	}

	// ── Workspace roadmaps ──────────────────────────────────────────────
	// Arsitektur 2026-04-25: setiap AI punya kamar sendiri di workspaces/<tugas>/roadmap/
	// dengan sub-folder daily/, weekly/, monthly/, yearly/.
	// Ingest ke documents type='roadmap' dengan title format "warga/<tugas>/<sub-path>"
	// supaya RoadmapHandler tree grouping (bersama/warga/archive) tetap bekerja.
	wsRoot := filepath.Join(projectRoot, "workspaces")
	if entries, err := os.ReadDir(wsRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			rmRoot := filepath.Join(wsRoot, entry.Name(), "roadmap")
			if _, err := os.Stat(rmRoot); err != nil {
				continue
			}
			_ = filepath.Walk(rmRoot, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
					return nil
				}
				relPath, _ := filepath.Rel(rmRoot, path)
				// Title format: "warga/<workspace-name>/<daily|weekly|monthly|yearly>/file.md"
				title := "warga/" + entry.Name() + "/" + filepath.ToSlash(relPath)
				content, err := fsutil.SafeReadFile(path)
				if err != nil {
					return nil
				}
				_, err = tx.Exec(`INSERT INTO documents (type, title, content, author)
					VALUES ('roadmap', ?, ?, ?)
					ON CONFLICT(type, title) DO UPDATE SET
						content=excluded.content,
						updated_at=CURRENT_TIMESTAMP`,
					title, string(content), entry.Name())
				if err == nil {
					count++
				} else {
					log.Printf("fq-brain: error inserting workspace roadmap %s: %v", title, err)
				}
				return nil
			})
		}
	}

	// Single-file fallbacks: docs/changelog.md, docs/inventory.md — overwrite
	// setelah folder supaya row single-file di-prioritaskan kalau ada konflik
	// title. Title-nya dibikin unique biar ga bentrok sama folder-version.
	singleFileMap := map[string]string{
		"changelog.md":               "changelog",
		"inventory.md":               "inventory",
		"README.md":                  "general",
		"floworkos_deep_analysis.md": "general",
	}
	for filename, docType := range singleFileMap {
		path := filepath.Join(docsRoot, filename)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		content, err := fsutil.SafeReadFile(path)
		if err != nil {
			continue
		}
		_, err = tx.Exec(`INSERT INTO documents (type, title, content, author)
			VALUES (?, ?, ?, 'system')
			ON CONFLICT(type, title) DO UPDATE SET
				content=excluded.content,
				updated_at=CURRENT_TIMESTAMP`,
			docType, filename, string(content))
		if err == nil {
			count++
		} else {
			log.Printf("fq-brain: error inserting %s: %v", filename, err)
		}
	}
	// ── State changelog files ───────────────────────────────────────────
	// Changelog juga bisa ditulis langsung ke state/changelog/*.md (oleh AI
	// atau script). Ingest ke documents type='changelog' supaya GUI bisa render.
	clRoot := filepath.Join(projectRoot, "state", "changelog")
	if _, err := os.Stat(clRoot); err == nil {
		_ = filepath.Walk(clRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			content, err := fsutil.SafeReadFile(path)
			if err != nil {
				return nil
			}
			_, err = tx.Exec(`INSERT INTO documents (type, title, content, author)
				VALUES ('changelog', ?, ?, 'system')
				ON CONFLICT(type, title) DO UPDATE SET
					content=excluded.content,
					updated_at=CURRENT_TIMESTAMP`,
				info.Name(), string(content))
			if err == nil {
				count++
			}
			return nil
		})
	}

	// Bug docs → immune_system (schema khusus untuk bug tracking).
	bugRoot := filepath.Join(docsRoot, "bug")
	if _, err := os.Stat(bugRoot); err == nil {
		err = filepath.Walk(bugRoot, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}
			title := info.Name()
			content, err := fsutil.SafeReadFile(path)
			if err != nil {
				return nil
			}
			desc := title
			for _, line := range strings.Split(string(content), "\n") {
				if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
					desc = strings.TrimSpace(line)
					break
				}
			}
			_, err = tx.Exec(`INSERT INTO immune_system (type, name, status, severity, description, meta_data)
				VALUES ('bug_doc', ?, 'open', 'info', ?, ?)
				ON CONFLICT(type, name) DO UPDATE SET
					description=excluded.description,
					meta_data=excluded.meta_data,
					updated_at=CURRENT_TIMESTAMP`,
				title, desc, string(content))
			if err == nil {
				count++
			}
			return nil
		})
	}

	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("commit: %w", err)
	}
	log.Printf("fq-brain: ingested %d docs (all categories)", count)
	return count, nil
}
