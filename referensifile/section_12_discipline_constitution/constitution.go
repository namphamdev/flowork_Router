// Package ingestor — constitution.go
// Injector konstitusi: parse doktrin FloworkOS dari tabel prompt_templates
// menjadi baris-baris tabel constitution di SQLite dengan amplitude immutable.
//
// Ayah 2026-04-24: source of truth sekarang `prompt_templates` table (sebelumnya
// file .md di promp/). File fallback tetap buat bootstrapping kalau DB belum
// populated — scripts/migrate_prompts.go bakal populate dari .md awalnya.
package ingestor

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
)

// constitutionSource mendefinisikan satu doktrin yang akan di-ingest.
type constitutionSource struct {
	TemplateName string  // LOWER(prompt_templates.name), matches basename .md lowercased
	RelPath      string  // path relatif — dipakai sebagai source_file label di tabel constitution
	Amplitude    float64 // amplitude default per section
}

// constitutionSources — urutan doktrin yang akan di-parse.
// Amplitude 999999 = immutable (tidak terdampak decay).
var constitutionSources = []constitutionSource{
	{"soul", "promp/SOUL.md", 999999.0},
	{"agents", "promp/AGENTS.md", 999999.0},
	{"prinsip_flowork_kuantum", "promp/PRINSIP_FLOWORK_KUANTUM.md", 999999.0},
	{"gol_flowork", "promp/GOL_FLOWORK.MD", 999999.0},
	{"adr-017-standar-agent", "promp/ADR-017-standar-agent.md", 999999.0},
	{"inventory", "docs/inventory.md", 500000.0},
	// DOKTRIN_MANDIRI: 8 hukum kemandirian warga. Amplitude 999998 (tepat di
	// bawah SOUL 999999). Source of truth = file ini, seeded ke constitution table.
	{"doktrin_mandiri", "floworkos-go/docs/DOKTRIN_MANDIRI.md", 999998.0},
}

// IngestConstitution memparse doktrin FloworkOS dari tabel prompt_templates
// (kalau ada) atau file fallback, lalu ingest sebagai section ke tabel
// constitution. Setiap heading ## jadi satu section.
func IngestConstitution(db *sql.DB, projectRoot string) (int, error) {
	count := 0
	for _, src := range constitutionSources {
		content := loadConstitutionContent(db, projectRoot, src)
		if content == "" {
			continue
		}

		sections := splitBySections(content)
		for _, sec := range sections {
			if strings.TrimSpace(sec.Content) == "" {
				continue
			}

			_, err := db.Exec(`INSERT INTO constitution (source_file, section, content, amplitude)
				VALUES (?, ?, ?, ?)
				ON CONFLICT DO NOTHING`,
				src.RelPath, sec.Heading, sec.Content, src.Amplitude)
			if err == nil {
				count++
			}
		}
	}

	// Tambahkan standar kerja agent sebagai section khusus
	count += ingestWorkProtocol(db)

	return count, nil
}

// loadConstitutionContent resolve konten doktrin. Priority:
//  1. prompt_templates WHERE LOWER(name)=TemplateName — canonical
//  2. File fallback projectRoot/RelPath — emergency bootstrap
//
// Return empty string kalau dua-duanya ga ketemu.
func loadConstitutionContent(db *sql.DB, projectRoot string, src constitutionSource) string {
	var tmplContent string
	if err := db.QueryRow(
		"SELECT content FROM prompt_templates WHERE LOWER(name) = ?", src.TemplateName,
	).Scan(&tmplContent); err == nil && strings.TrimSpace(tmplContent) != "" {
		return tmplContent
	}
	absPath := filepath.Join(projectRoot, filepath.FromSlash(src.RelPath))
	if data, err := os.ReadFile(absPath); err == nil {
		return string(data)
	}
	return ""
}

// section merepresentasikan satu blok di markdown file.
type section struct {
	Heading string
	Content string
}

// splitBySections memecah markdown content berdasarkan heading ##.
// Heading # (level 1) dipakai sebagai prefix context.
func splitBySections(content string) []section {
	lines := strings.Split(content, "\n")
	var sections []section
	var currentHeading string
	var currentContent strings.Builder
	var titlePrefix string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			// Level 1 heading — jadikan prefix
			titlePrefix = strings.TrimPrefix(trimmed, "# ")
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			// Flush previous section
			if currentHeading != "" {
				sections = append(sections, section{
					Heading: currentHeading,
					Content: strings.TrimSpace(currentContent.String()),
				})
			}
			heading := strings.TrimPrefix(trimmed, "## ")
			// Tambah prefix dari title
			if titlePrefix != "" {
				currentHeading = titlePrefix + " > " + heading
			} else {
				currentHeading = heading
			}
			currentContent.Reset()
			continue
		}

		currentContent.WriteString(line)
		currentContent.WriteString("\n")
	}

	// Flush terakhir
	if currentHeading != "" {
		sections = append(sections, section{
			Heading: currentHeading,
			Content: strings.TrimSpace(currentContent.String()),
		})
	}

	// Jika tidak ada ## heading, masukkan sebagai satu section utuh
	if len(sections) == 0 && strings.TrimSpace(content) != "" {
		heading := titlePrefix
		if heading == "" {
			heading = "Full Document"
		}
		sections = append(sections, section{
			Heading: heading,
			Content: strings.TrimSpace(content),
		})
	}

	return sections
}

// ingestWorkProtocol memasukkan standar kerja agent ke constitution.
func ingestWorkProtocol(db *sql.DB) int {
	protocol := `STANDAR KERJA WAJIB SEMUA AGENT:
1. Buat roadmap sendiri → analisa tugas, breakdown jadi langkah kecil
2. Buat task list → TodoWrite / checklist eksplisit
3. Eksekusi → kerjakan spesifik per task, jangan menyimpang
4. Test → go build, go test, verifikasi visual. WAJIB LULUS
5. Update task → centang yang sudah selesai
6. Lanjut ke bagian selanjutnya → ulangi step 3-5
7. Selesai? → Review ulang semua perubahan
8. Cari bug → scan kode yang diubah
9. Ada bug? → Perbaiki, kembali ke step 4
10. Gagal terus? → Cari solusi di brain (knowledge base / recordings)
11. Berhasil? → INPUT PENGALAMAN + SOLUSI ke otak brain
12. JANGAN input kegagalan ke otak — itu noise, bukan signal

PRINSIP: Contek cara kerja Antigravity (plan → execute → verify → memorize → report).
Setiap agen WAJIB mematuhi ini tanpa pengecualian.`

	_, err := db.Exec(`INSERT INTO constitution (source_file, section, content, amplitude)
		VALUES (?, ?, ?, ?)
		ON CONFLICT DO NOTHING`,
		"RUNTIME", "Standar Kerja Agent (Work Protocol)", protocol, 999999.0)
	if err != nil {
		return 0
	}
	return 1
}
