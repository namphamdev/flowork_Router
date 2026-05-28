package i18n

import "testing"

func TestAvailableTags_AllLanguagesShipped(t *testing.T) {
	tags := AvailableTags()
	want := map[string]bool{"en": false, "ja-JP": false, "vi": false, "zh-CN": false}
	for _, tag := range tags {
		want[tag] = true
	}
	for tag, found := range want {
		if !found {
			t.Fatalf("locale %q not shipped", tag)
		}
	}
}

func TestT_ResolvesPerLanguage(t *testing.T) {
	if T("ja-JP", "nav.endpoint") != "エンドポイント" {
		t.Fatal("ja-JP nav.endpoint mismatch")
	}
	if T("vi", "action.save") != "Lưu" {
		t.Fatal("vi action.save mismatch")
	}
	if T("zh-CN", "status.success") != "成功" {
		t.Fatal("zh-CN status.success mismatch")
	}
}

func TestT_FallsBackToEnglish(t *testing.T) {
	// Unknown locale → en
	if T("klingon", "action.save") != "Save" {
		t.Fatal("unknown locale should fallback to en")
	}
	// Empty key → key itself
	if T("en", "missing.key") != "missing.key" {
		t.Fatal("missing key should echo key")
	}
}

func TestCatalog_ReturnsCopy(t *testing.T) {
	c1 := Catalog("en")
	c1["nav.endpoint"] = "polluted"
	c2 := Catalog("en")
	if c2["nav.endpoint"] == "polluted" {
		t.Fatal("Catalog must return a defensive copy")
	}
}
