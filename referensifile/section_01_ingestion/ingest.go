// Package ingestor memindai filesystem FloworkOS dan mempopulasi tabel
// agents, skills, dan memories di FQ-Brain SQLite.
//
// Semua fungsi bersifat idempotent (upsert, bukan insert ganda).
// File .md asli TIDAK dihapus — hanya ditandai dengan header BRAIN SYNC.
package ingestor

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/teetah2402/flowork/internal/wargaregistry"
)

// IngestAll menjalankan semua ingestor secara berurutan.
// Urutan: agents → enrich dari prompts → skills → memories → constitution.
func IngestAll(db *sql.DB, projectRoot string) error {
	n1, err := IngestAgents(db, projectRoot)
	if err != nil {
		return fmt.Errorf("ingest agents: %w", err)
	}
	// Enrich agents dengan daemon info dari cmd/daemons/
	enrichAgentsWithDaemons(db, projectRoot)

	n2, err := IngestSkills(db, projectRoot)
	if err != nil {
		return fmt.Errorf("ingest skills: %w", err)
	}
	n3, err := IngestMemories(db, projectRoot)
	if err != nil {
		return fmt.Errorf("ingest memories: %w", err)
	}
	n4, err := IngestConstitution(db, projectRoot)
	if err != nil {
		return fmt.Errorf("ingest constitution: %w", err)
	}
	n5, err := IngestModelPool(db, projectRoot)
	if err != nil {
		return fmt.Errorf("ingest model pool: %w", err)
	}
	n6, err := IngestDocuments(db, projectRoot)
	if err != nil {
		return fmt.Errorf("ingest documents: %w", err)
	}
	fmt.Fprintf(os.Stderr, "fq-brain: ingested %d agents, %d skills, %d memories, %d constitution, %d models, %d documents\n", n1, n2, n3, n4, n5, n6)
	return nil
}

// IngestAgents memindai workspaces/ dan skills/ untuk membangun tabel agents.
// Semua nama agent diambil dari filesystem — TIDAK ada hardcode.
func IngestAgents(db *sql.DB, projectRoot string) (int, error) {
	workspacesDir := filepath.Join(projectRoot, "workspaces")
	entries, err := os.ReadDir(workspacesDir)
	if err != nil {
		return 0, nil // no workspaces dir = skip
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx agents: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Production policy 2026-05-06: filter strict — workspaces/ folder
		// banyak yang isinya task artifact (chat, coder, qc, refactor, dll),
		// BUKAN warga directory. Sebelumnya semua sub-folder otomatis jadi
		// agent row → DB bloat 18+ row tiap boot. Sekarang skip kecuali:
		//   - existing agent row (preserve, biar ingestor tetap update meta), ATAU
		//   - folder punya marker file (system_prompt.md ATAU agent.md ATAU README warga marker)
		var existingID int
		_ = tx.QueryRow("SELECT id FROM agents WHERE name = ?", name).Scan(&existingID)
		if existingID == 0 {
			// New folder, cek marker
			markerExists := false
			for _, marker := range []string{"system_prompt.md", "agent.md", "warga.md", "persona.md"} {
				if _, err := os.Stat(filepath.Join(workspacesDir, name, marker)); err == nil {
					markerExists = true
					break
				}
			}
			if !markerExists {
				continue // task folder, bukan warga — skip
			}
		}
		displayName := strings.Title(name) //nolint:staticcheck

		// Scan skills/ untuk menentukan role dari nama folder skill
		role := ""
		skillsDir := filepath.Join(projectRoot, "skills")
		skillEntries, _ := os.ReadDir(skillsDir)
		for _, se := range skillEntries {
			if se.IsDir() && strings.HasPrefix(se.Name(), name+"-") {
				role = strings.TrimPrefix(se.Name(), name+"-")
				break
			}
		}

		wsPath := filepath.Join("workspaces", name)
		envPrefix := "FLOWORK_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_"

		// Fix 2026-04-24: jangan override status ke 'active' tiap boot.
		// Sebelumnya user toggle OFF di GUI tab Warga, ingester re-activate
		// agent ghost (coding, telegram) setiap boot flowork-gui. Akibatnya
		// Ayah lapor "semua AI gw nonaktifkan tapi dia masih bisa balas chat".
		// Sekarang:
		//   - INSERT row baru default 'idle' (ayah harus eksplisit aktifkan).
		//   - ON CONFLICT preserve status existing (user intent sakral).
		_, err := tx.Exec(`INSERT INTO agents (name, display_name, role, workspace_path, env_prefix, status)
			VALUES (?, ?, ?, ?, ?, 'idle')
			ON CONFLICT(name) DO UPDATE SET
				display_name=excluded.display_name,
				role=CASE WHEN agents.role='' THEN excluded.role ELSE agents.role END,
				workspace_path=excluded.workspace_path,
				env_prefix=excluded.env_prefix,
				updated_at=CURRENT_TIMESTAMP`,
			name, displayName, role, wsPath, envPrefix)
		if err != nil {
			log.Printf("fq-brain: failed to upsert agent %s: %v", name, err)
			continue
		}
		log.Printf("fq-brain: ingested agent: %s", name)
		count++
	}
	if err := tx.Commit(); err != nil { return count, fmt.Errorf("tx commit (agents): %w", err) }
	return count, nil
}

// enrichAgentsWithDaemons memindai cmd/daemons/ untuk menautkan daemon_cmd
// ke agent yang sesuai. Plug-and-play: nama daemon → nama agent.
func enrichAgentsWithDaemons(db *sql.DB, projectRoot string) {
	daemonsDir := filepath.Join(projectRoot, "cmd", "daemons")
	entries, err := os.ReadDir(daemonsDir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		daemonName := e.Name() // e.g. "flowork-telegram"

		// Map daemon name ke agent name:
		// "flowork-telegram" → "merpati" (Telegram bridge)
		// "flowork-discord" → "ombak" (Discord bridge)
		// "flowork-finance" → "wira-eka" (Finance/custody)
		// Generic fallback: strip "flowork-" prefix
		agentName := daemonToAgent(daemonName)

		_, err = db.Exec(`UPDATE agents SET
			daemon_cmd = CASE WHEN daemon_cmd != '' AND daemon_cmd != ? THEN daemon_cmd || ',' || ? ELSE ? END,
			updated_at = CURRENT_TIMESTAMP
			WHERE name = ?`,
			daemonName, daemonName, daemonName, agentName)
		if err != nil {
			log.Printf("fq-brain: enrich daemon %s error: %v", daemonName, err)
		} else {
			log.Printf("fq-brain: mapped daemon %s to agent %s", daemonName, agentName)
		}
	}
}

// daemonToAgent maps daemon binary names to agent identities.
// Delegates to wargaregistry.DaemonWarga — single source of truth (effekdomino #8).
func daemonToAgent(daemonName string) string {
	return wargaregistry.DaemonWarga(daemonName)
}

// IngestSkills memindai skills/*/SKILL.md dan memasukkan ke tabel skills.
func IngestSkills(db *sql.DB, projectRoot string) (int, error) {
	skillsDir := filepath.Join(projectRoot, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx skills: %w", err)
	}
	defer tx.Rollback()

	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		// Find SKILL.md or README.md
		skillFile := filepath.Join(skillsDir, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err != nil {
			skillFile = filepath.Join(skillsDir, e.Name(), "README.md")
			if _, err := os.Stat(skillFile); err != nil {
				continue
			}
		}

		content, err := os.ReadFile(skillFile)
		if err != nil {
			continue
		}

		// Parse agent name from dir name (e.g. "wangsit-ahli-arsitektur" → "wangsit")
		dirName := e.Name()
		agentName := dirName
		if idx := strings.Index(dirName, "-ahli-"); idx > 0 {
			agentName = dirName[:idx]
		} else if idx := strings.Index(dirName, "-"); idx > 0 {
			// fallback: first segment before dash
			parts := strings.SplitN(dirName, "-", 2)
			agentName = parts[0]
		}

		skillName := dirName

		// Get agent_id (create agent if not exists)
		var agentID int
		err = tx.QueryRow("SELECT id FROM agents WHERE name = ?", agentName).Scan(&agentID)
		if err != nil {
			// Agent not in workspaces — create minimal entry
			res, err2 := tx.Exec(`INSERT OR IGNORE INTO agents (name, display_name, role) VALUES (?, ?, ?)`,
				agentName, strings.Title(agentName), dirName) //nolint:staticcheck
			if err2 != nil {
				log.Printf("fq-brain: failed to create agent %s for skill %s: %v", agentName, skillName, err2)
				continue
			}
			id, _ := res.LastInsertId()
			if id == 0 {
				err = tx.QueryRow("SELECT id FROM agents WHERE name = ?", agentName).Scan(&agentID)
				if err != nil {
					log.Printf("fq-brain: failed to resolve agent %s after insert: %v", agentName, err)
					continue
				}
			} else {
				agentID = int(id)
			}
		}

		if agentID == 0 {
			continue
		}

		_, err = tx.Exec(`INSERT INTO skills (agent_id, skill_name, content, version)
			VALUES (?, ?, ?, 1)
			ON CONFLICT(agent_id, skill_name) DO UPDATE SET 
				content=excluded.content,
				version=skills.version+1,
				updated_at=CURRENT_TIMESTAMP`,
			agentID, skillName, string(content))
		if err == nil {
			log.Printf("fq-brain: ingested skill %s for agent %s", skillName, agentName)
			count++
		} else {
			log.Printf("fq-brain: error inserting skill %s: %v", skillName, err)
		}
	}
	if err := tx.Commit(); err != nil { return count, fmt.Errorf("tx commit (skills): %w", err) }
	return count, nil
}

// IngestMemories memindai memory/**/*.md dan memasukkan ke tabel memories.
func IngestMemories(db *sql.DB, projectRoot string) (int, error) {
	memoryDir := filepath.Join(projectRoot, "memory")
	if _, err := os.Stat(memoryDir); err != nil {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin tx memories: %w", err)
	}
	defer tx.Rollback()

	count := 0
	err = filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("fq-brain: walk error at %s: %v", path, err)
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		// Skip index/readme files
		lower := strings.ToLower(info.Name())
		if lower == "readme.md" || lower == "index.md" || lower == "memory.md" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("fq-brain: failed to read memory file %s: %v", path, err)
			return nil
		}

		// Determine agent and category from path
		relPath, _ := filepath.Rel(memoryDir, path)
		parts := strings.Split(filepath.ToSlash(relPath), "/")

		agentName := "shared"
		category := "general"
		title := strings.TrimSuffix(info.Name(), ".md")

		if len(parts) >= 2 {
			dirName := parts[0]
			// Check if dir is an agent name or a category
			switch dirName {
			case "shared", "pahlawan", "onboarding":
				agentName = "shared"
				category = dirName
			case "death-letters":
				agentName = "shared"
				category = "death-letter"
			case "doubts":
				agentName = "shared"
				category = "doubt"
			case "inheritance":
				agentName = "shared"
				category = "inheritance"
			default:
				agentName = dirName
				category = "journey"
			}
			// Deeper path → refine category
			if len(parts) >= 3 {
				category = parts[1]
			}
		} else {
			// Root level files like feedback_*, project_*, reference_*, user_*
			if strings.HasPrefix(title, "feedback_") {
				category = "feedback"
			} else if strings.HasPrefix(title, "project_") {
				category = "project"
			} else if strings.HasPrefix(title, "reference_") {
				category = "reference"
			} else if strings.HasPrefix(title, "user_") {
				category = "user-profile"
			}
		}

		// Get or create agent_id
		var agentID int
		err = tx.QueryRow("SELECT id FROM agents WHERE name = ?", agentName).Scan(&agentID)
		if err != nil {
			res, err2 := tx.Exec(`INSERT OR IGNORE INTO agents (name, display_name, role) VALUES (?, ?, '')`,
				agentName, strings.Title(agentName)) //nolint:staticcheck
			if err2 != nil {
				log.Printf("fq-brain: failed to create agent %s for memory %s: %v", agentName, title, err2)
				return nil
			}
			id, _ := res.LastInsertId()
			if id == 0 {
				err = tx.QueryRow("SELECT id FROM agents WHERE name = ?", agentName).Scan(&agentID)
				if err != nil {
					log.Printf("fq-brain: failed to resolve agent %s for memory %s: %v", agentName, title, err)
					return nil
				}
			} else {
				agentID = int(id)
			}
		}

		if agentID == 0 {
			return nil
		}

		srcPath := filepath.ToSlash(relPath)
		_, err = tx.Exec(`INSERT INTO memories (agent_id, category, title, content, source_path)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(agent_id, source_path) DO UPDATE SET
				content=excluded.content,
				updated_at=CURRENT_TIMESTAMP`,
			agentID, category, title, string(content), srcPath)
		if err == nil {
			log.Printf("fq-brain: ingested memory %s for agent %s", title, agentName)
			count++
		} else {
			log.Printf("fq-brain: error inserting memory %s: %v", title, err)
		}
		return nil
	})
	if err == nil {
		if err := tx.Commit(); err != nil { return count, fmt.Errorf("tx commit (memories): %w", err) }
	}
	return count, err
}
