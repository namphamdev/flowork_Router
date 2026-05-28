---
name: qc_three_layer
description: Pilar 3 workflow — QC 3-lapis (L1 pre-check + L2 automated + L3 Mr.Dev manual chat judge).
metadata:
  domain: mindset
  created_at: 2026-05-19
---

# QC Three-Layer Workflow (Pilar 3)

## When to Use
- Sebelum claim fitur baru "WORK / IMPLEMENTED"
- Sebelum mark `✅ IMPLEMENTED` di README / changelog
- Pas Mr.Dev minta verifikasi fitur

## 3 Lapis QC

### L1 Pre-check (Artifact Verify)
**Fungsi:** Sanity check artifact ada + struktural valid.

- File path exists?
- JSON/YAML valid parse?
- Schema migration applied?
- Kernel/service reachable (HTTP ping)?
- Brain entry inserted (SQL COUNT)?

**Output:** exit 0 / error log

**Cukup untuk claim WORK?** ❌ TIDAK

### L2 Automated Dispatch (Cluster Scoring)
**Fungsi:** Real dispatch ke runtime, cluster classify per behavior.

- Dispatch via `/v1/chat` (kernel-mediated, BUKAN `/v1/tool/execute` direct)
- Tool invoke + side-effect verify (unique marker → query DB → confirm artifact)
- Cluster 6 kategori: PASS / VALID / FAIL-REFUSED / HALU / HALU-VERIFIED / FAIL-INFRA
- Threshold: PASS+VALID ≥80%, HALU+HALU-VERIFIED <5%

**Output:** ratio per cluster + JSON report

**Cukup untuk claim WORK?** ❌ TIDAK (cuma indikasi)

### L3 Mr.Dev Manual Chat Judge
**Fungsi:** Mr.Dev interact langsung, observe behavior real, verdict verbatim.

- Mr.Dev ngetik query natural
- Observe response, tone, edge case
- Verdict text (POSITIVE / NEGATIVE / ITERATE)
- Record di `qc/qc-YYYY-MM-DD-<topic>.md` (append-only)

**Output:** Verdict verbatim Mr.Dev

**Cukup untuk claim WORK?** ✅ YA (cumulative dengan L1+L2)

## Default Status Fitur Baru
- **STAGED** — artifact ada, kode jalan, brain entry inserted (sampai L3 lulus)
- **IMPLEMENTED** — HANYA setelah L1 + L2 + L3 cumulative PASS

## Decision Tree

```
Fitur baru
   ↓
L1 PASS? ─NO→ Fix artifact issue, retry
   ↓ YES
L2 ≥80%? ─NO→ Identify failure cluster, iterate
   ↓ YES
L3 Mr.Dev verdict POSITIVE? ─NO→ Revert claim, root cause, iterate
   ↓ YES
Promote STAGED → IMPLEMENTED + update README + changelog + record qc/
```

## Anti-Pattern
- ❌ Script PASS → mark IMPLEMENTED tanpa L3
- ❌ Mr.Dev thumbs up 1x → assume semua use case
- ❌ Skip L3 karena "udah kelihatan jalan"
- ❌ Tulis "✅ IMPLEMENTED" padahal L3 belum jalan

## Trigger Self-Audit
Sebelum tulis "✅ IMPLEMENTED":
- L1 done? L2 ≥80%? L3 Mr.Dev verdict POSITIVE?
- Kalau ada 1 NO → tulis "⏳ STAGED + alasan"

## References
- [[qc-three-layer-standard]] (memory)
- [[flowork-4-pillars-master]] Pilar 3
- Case study: `qc/qc-pending-post-v10.md` (Wave 2 staged, L3 pending)
