// Package mesh — crdt_merge.go: M04.5 Conflict Resolution.
//
// Per ROADMAP_AKTIF Tier 4 M04.5: kalau 2 peer offline → online lagi, write
// terhadap row sama → konflik. Resolver pakai Last-Write-Wins (LWW)
// timestamp + tie-break peer karma score.
//
// Algorithm (deterministic):
//   1. Compare timestamp — newer wins
//   2. Tie (same timestamp) → compare karma — higher karma wins
//   3. Tie (same karma) → compare peer ID lexicographically (stable order)
//
// Peer karma source: kernel/mesh/karma.go::PeerKarma()
package mesh

// MergeOp represents a write operation candidate for conflict resolution.
type MergeOp struct {
	PeerID    string  // peer identifier (Ed25519 pub key hex atau alias)
	Timestamp int64   // Unix nanos
	Karma     float64 // peer karma at write time (0.0-1.0)
	Payload   []byte  // actual write content (opaque to resolver)
}

// Resolve picks winner from candidate ops. Returns winning op + reason.
//
// Returns nil + "" kalau ops kosong.
//
// Tie-break order:
//   1. timestamp DESC (newer wins)
//   2. karma DESC (high-karma peer wins)
//   3. peer_id ASC (deterministic, lexicographic)
func Resolve(ops []MergeOp) (*MergeOp, string) {
	if len(ops) == 0 {
		return nil, ""
	}
	if len(ops) == 1 {
		return &ops[0], "single_candidate"
	}

	winner := &ops[0]
	reason := "lww"
	for i := 1; i < len(ops); i++ {
		c := &ops[i]
		switch {
		case c.Timestamp > winner.Timestamp:
			winner = c
			reason = "lww"
		case c.Timestamp < winner.Timestamp:
			// keep winner
		case c.Karma > winner.Karma:
			// timestamp tie, karma higher wins
			winner = c
			reason = "karma_tiebreak"
		case c.Karma < winner.Karma:
			// keep winner
		default:
			// karma tie, peer_id lexicographic ASC
			if c.PeerID < winner.PeerID {
				winner = c
				reason = "peer_id_tiebreak"
			}
		}
	}
	return winner, reason
}

// AllResolveDeterministic — verify same input → same output (called by tests).
// Production: peer can independently call Resolve dengan ops set sama, akan
// dapet winner sama → no need for coordination.
func AllResolveDeterministic(ops []MergeOp, iterations int) bool {
	first, _ := Resolve(ops)
	for i := 0; i < iterations; i++ {
		got, _ := Resolve(ops)
		if (first == nil) != (got == nil) {
			return false
		}
		if first != nil && got != nil && first.PeerID != got.PeerID {
			return false
		}
	}
	return true
}
