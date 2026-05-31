package mesh

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func hashHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestTrigramJaccard(t *testing.T) {
	if s := TrigramJaccard("hello world", "hello world"); s != 1.0 {
		t.Errorf("identical should be 1.0, got %v", s)
	}
	// Normalization: case + punctuation ignored → still identical.
	if s := TrigramJaccard("Hello, WORLD!", "hello world"); s != 1.0 {
		t.Errorf("normalized-identical should be 1.0, got %v", s)
	}
	// Reworded near-dup should score high (above dedup threshold).
	if s := TrigramJaccard("the quick brown fox jumps", "the quick brown fox leaps"); s < 0.5 {
		t.Errorf("near-dup should score high, got %v", s)
	}
	// Totally different → low.
	if s := TrigramJaccard("apple pie recipe", "quantum chromodynamics"); s > 0.2 {
		t.Errorf("different should score low, got %v", s)
	}
	// Two empties are "the same"; empty vs non-empty is 0.
	if s := TrigramJaccard("", ""); s != 1.0 {
		t.Errorf("empty==empty should be 1.0, got %v", s)
	}
	if s := TrigramJaccard("", "x"); s != 0.0 {
		t.Errorf("empty vs non-empty should be 0.0, got %v", s)
	}
}

func TestVectorClock(t *testing.T) {
	a := VectorClock{}
	b := VectorClock{}
	a.Tick("n1")
	a.Tick("n1") // a: n1=2
	b.Tick("n2") // b: n2=1
	if a.Compare(b) != ClockConcurrent {
		t.Errorf("a vs b should be concurrent")
	}
	merged := VectorClock{}
	merged.Merge(a)
	merged.Merge(b) // n1=2, n2=1
	if a.Compare(merged) != ClockBefore {
		t.Errorf("a should be before merged")
	}
	if merged.Compare(a) != ClockAfter {
		t.Errorf("merged should be after a")
	}
	c := VectorClock{"n1": 2, "n2": 1}
	if merged.Compare(c) != ClockEqual {
		t.Errorf("merged should equal c")
	}
}

func TestGSet(t *testing.T) {
	a := NewGSet()
	a.Add("x")
	a.Add("y")
	b := NewGSet()
	b.Add("y")
	b.Add("z")
	a.Merge(b)
	got := a.Elements() // sorted
	want := []string{"x", "y", "z"}
	if len(got) != 3 || got[0] != want[0] || got[2] != want[2] {
		t.Errorf("GSet merge = %v, want %v", got, want)
	}
}

func TestTwoPhaseSet(t *testing.T) {
	s := NewTwoPhaseSet()
	s.Add("a")
	s.Add("b")
	s.Remove("a")
	if s.Has("a") {
		t.Error("a was removed, should be absent")
	}
	if !s.Has("b") {
		t.Error("b should be present")
	}
	// Merge commutativity: removes win regardless of order.
	other := NewTwoPhaseSet()
	other.Add("a") // re-add attempt
	s.Merge(other)
	if s.Has("a") {
		t.Error("2P-Set: removed element must stay removed after merge (remove wins)")
	}
}

func TestValidateToolManifest(t *testing.T) {
	ok, _ := ValidateToolManifest(`{"name":"weather","description":"get weather"}`)
	if !ok {
		t.Error("clean manifest should pass")
	}
	if ok, _ := ValidateToolManifest(`{"description":"no name"}`); ok {
		t.Error("missing name should fail")
	}
	if ok, _ := ValidateToolManifest(`not json`); ok {
		t.Error("non-json should fail")
	}
	if ok, reason := ValidateToolManifest(`{"name":"evil","cmd":"rm -rf /"}`); ok {
		t.Errorf("dangerous token should fail, got ok (reason=%q)", reason)
	}
	if ok, _ := ValidateToolManifest(`{"name":"x","run":"os.system('id')"}`); ok {
		t.Error("os.system should fail")
	}
}

func TestValidateLoraDelta(t *testing.T) {
	if ok, _ := ValidateLoraDelta("m", "https://host/d.bin", 1024, "sig"); !ok {
		t.Error("valid delta should pass")
	}
	if ok, _ := ValidateLoraDelta("", "https://x", 10, "s"); ok {
		t.Error("missing model should fail")
	}
	if ok, _ := ValidateLoraDelta("m", "ftp://x", 10, "s"); ok {
		t.Error("bad scheme should fail")
	}
	if ok, _ := ValidateLoraDelta("m", "https://x", MaxLoraDeltaBytes+1, "s"); ok {
		t.Error("oversize should fail")
	}
	if ok, _ := ValidateLoraDelta("m", "https://x", 10, ""); ok {
		t.Error("missing signature should fail")
	}
	// file:// allowed for sneakernet.
	if ok, _ := ValidateLoraDelta("m", "file:///mnt/usb/d.bin", 10, "s"); !ok {
		t.Error("file:// should be allowed")
	}
}

func TestVerifyDeltaChecksum(t *testing.T) {
	data := []byte("hello lora")
	ok, _ := VerifyDeltaChecksum(data, hashHex(data))
	if !ok {
		t.Error("matching checksum should verify")
	}
	if ok, _ := VerifyDeltaChecksum(data, "deadbeef"); ok {
		t.Error("mismatched checksum should fail")
	}
	if ok, _ := VerifyDeltaChecksum(data, ""); ok {
		t.Error("empty expected checksum should fail-closed")
	}
}

func TestApplyLoraDeltaDeferred(t *testing.T) {
	if err := ApplyLoraDelta("m", "/tmp/x"); err != ErrLoraApplyUnavailable {
		t.Errorf("apply should be honestly deferred, got %v", err)
	}
}

func TestExtractDrawerContent(t *testing.T) {
	if c := extractDrawerContent(`{"drawer_content":"hi"}`); c != "hi" {
		t.Errorf("drawer_content extract = %q", c)
	}
	if c := extractDrawerContent(`{"content":"yo"}`); c != "yo" {
		t.Errorf("content extract = %q", c)
	}
	if c := extractDrawerContent(`raw text`); c != "raw text" {
		t.Errorf("raw fallback = %q", c)
	}
}
