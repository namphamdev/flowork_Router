// Package ingestor — brain_reset.go
// Menghapus total semua data di 8 tabel brain SQLite.
// Digunakan saat ingin bootstrap ulang otak dari nol.
package ingestor

import (
	"database/sql"
	"fmt"
	"log"
)

// brainTables adalah urutan penghapusan (child dulu, parent terakhir).
// Sprint 3.5c (BUG-114/208 fix): hapus "entanglements" + "atoms" — tabel ini
// udah di-DROP sejak rc189 (lihat schema.go:77-78). Reset gagal di tengah
// jalan kalau coba DELETE dari tabel yang ngga exist → tabel sesudahnya
// (`agents`) ngga ke-reset → "Reset Brain" parsial, data agent lama bocor.
var brainTables = []string{
	"memories",
	"skills",
	"tool_patterns",
	"recordings",
	"constitution",
	"model_pool",
	"agents",
}

// ResetBrain menghapus SEMUA data di 8 tabel brain.
// Mengembalikan map[table]rowsDeleted untuk logging.
func ResetBrain(db *sql.DB) (map[string]int64, error) {
	result := make(map[string]int64)

	for _, table := range brainTables {
		// Count dulu untuk logging
		var count int64
		db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)

		_, err := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			return result, fmt.Errorf("reset table %s: %w", table, err)
		}
		result[table] = count
		if count > 0 {
			log.Printf("fq-brain: RESET %s — %d rows deleted", table, count)
		}
	}

	// Vacuum untuk reclaim disk space
	db.Exec("VACUUM")

	log.Printf("fq-brain: RESET COMPLETE — otak bersih, siap re-ingest")
	return result, nil
}
