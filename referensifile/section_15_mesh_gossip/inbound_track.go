// Package mesh — kernel/mesh/inbound_track.go
//
// Track peer yang push masuk via POST /v1/p2p/crdt/sync. Pattern asimetris
// bikin kita ngga punya MESH_PEER_SEEDS untuk Node B (di belakang NAT),
// jadi outbound discover ngga liat mereka. Tapi mereka push masuk —
// kita catat di sini biar GUI bisa nampilin "ada 2 node yang lagi sync
// sama gw, terakhir kontak X menit lalu".
//
// Tracking key = node_id (dari HLC event pertama). RemoteAddr disimpan
// untuk informasi tapi BUKAN sebagai key — peer bisa pindah IP (NAT
// rebalance, mobile network) tapi node_id stabil.
//
// Stale: kalau LastSeen > 30 menit lalu → mark Idle. Ngga di-evict
// (Ayah mungkin tetap mau lihat history "linux1 terakhir push 2 jam lalu").

package mesh

import (
	"sync"
	"time"
)

// InboundPeerStatus snapshot peer yang push CRDT ke kita.
type InboundPeerStatus struct {
	NodeID        string    `json:"node_id"`
	RemoteAddr    string    `json:"remote_addr,omitempty"`
	LastSeen      time.Time `json:"last_seen"`
	EventsTotal   int       `json:"events_total"`    // total event received cumulative
	LastBatchSize int       `json:"last_batch_size"` // size last push
	SyncCount     int       `json:"sync_count"`      // berapa kali push attempt sukses
}

// inboundIdleThreshold peer dianggap idle (bukan dead) kalau ngga kontak
// lebih dari ini. Ngga di-evict supaya history terlihat.
var inboundIdleThreshold = 30 * time.Minute

var (
	inboundMu      sync.RWMutex
	inboundByNode  = make(map[string]*InboundPeerStatus) // node_id → status
)

// RecordInbound catat satu push dari peer. Dipanggil dari handleCRDTSync
// setelah Apply sukses. nodeID dari HLC[2] event pertama (kalau ada),
// fallback ke remoteAddr kalau events kosong.
//
// batchSize = jumlah events yang baru di-push (bukan kumulatif).
//
// Dedupe: peer yang awalnya push body kosong (ke-track sebagai IP) lalu
// push dengan event proper (HLC.NodeID terisi) — kita merge stats lama
// dari entry IP ke entry node_id real, lalu hapus entry IP supaya GUI
// ngga tampil duplikat.
func RecordInbound(nodeID, remoteAddr string, batchSize int) {
	realID := nodeID
	if realID == "" {
		realID = remoteAddr
	}
	if realID == "" {
		return // ngga bisa identify, skip tracking
	}

	inboundMu.Lock()
	defer inboundMu.Unlock()

	// Dedupe: peer punya real nodeID sekarang, tapi sebelumnya pernah
	// ke-track sebagai IP-fallback (key=remoteAddr). Merge stats + delete
	// entry IP biar GUI ngga tampil 2 row buat node yang sama.
	if nodeID != "" && remoteAddr != "" && nodeID != remoteAddr {
		if oldEntry, hasOld := inboundByNode[remoteAddr]; hasOld {
			target, hasTarget := inboundByNode[nodeID]
			if !hasTarget {
				target = &InboundPeerStatus{NodeID: nodeID}
				inboundByNode[nodeID] = target
			}
			target.SyncCount += oldEntry.SyncCount
			target.EventsTotal += oldEntry.EventsTotal
			if oldEntry.LastSeen.After(target.LastSeen) {
				target.LastSeen = oldEntry.LastSeen
			}
			delete(inboundByNode, remoteAddr)
		}
	}

	st, ok := inboundByNode[realID]
	if !ok {
		st = &InboundPeerStatus{NodeID: realID}
		inboundByNode[realID] = st
	}
	st.RemoteAddr = remoteAddr
	st.LastSeen = time.Now().UTC()
	st.EventsTotal += batchSize
	st.LastBatchSize = batchSize
	st.SyncCount++
}

// InboundPeers return snapshot semua peer yang pernah push ke kita.
// Sorted by LastSeen desc (paling baru duluan).
func InboundPeers() []InboundPeerStatus {
	inboundMu.RLock()
	defer inboundMu.RUnlock()
	out := make([]InboundPeerStatus, 0, len(inboundByNode))
	for _, st := range inboundByNode {
		out = append(out, *st)
	}
	// Simple sort: bubble by LastSeen desc — list small (<10 typical)
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].LastSeen.After(out[i].LastSeen) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// IsInboundIdle return true kalau peer ngga kontak dalam window threshold.
// Helper untuk renderer GUI yang mau decide warna kartu (hijau aktif vs
// kuning idle vs abu-abu old).
func IsInboundIdle(st InboundPeerStatus) bool {
	if st.LastSeen.IsZero() {
		return true
	}
	return time.Since(st.LastSeen) > inboundIdleThreshold
}
