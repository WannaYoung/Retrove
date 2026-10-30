package translate

import (
	"strings"
	"testing"
)

func TestParseTranslationMapObject(t *testing.T) {
	m, err := parseTranslationMap(`{"12":"超级马里奥","13":"塞尔达传说"}`)
	if err != nil {
		t.Fatal(err)
	}
	if m[12] != "超级马里奥" || m[13] != "塞尔达传说" {
		t.Fatalf("%#v", m)
	}
}

func TestParseTranslationMapFenced(t *testing.T) {
	m, err := parseTranslationMap("```json\n{\"1\":\"洛克人\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if m[1] != "洛克人" {
		t.Fatalf("%#v", m)
	}
}

func TestPickSourcePrefersChinese(t *testing.T) {
	got := pickSource("Super Mario Bros.", "超级马里奥", "foo.zip")
	if got != "超级马里奥" {
		t.Fatal(got)
	}
}

func TestLooksMostlyChinese(t *testing.T) {
	if !looksMostlyChinese("超级马里奥") {
		t.Fatal("expected chinese")
	}
	if looksMostlyChinese("Super Mario") {
		t.Fatal("expected english")
	}
}

func TestLooksEnglish(t *testing.T) {
	if !looksEnglish("Super Mario World") {
		t.Fatal("expected english")
	}
	if looksEnglish("超级马里奥世界") {
		t.Fatal("expected chinese not english")
	}
	if looksEnglish("") {
		t.Fatal("empty")
	}
}

func TestPlatformHintKnown(t *testing.T) {
	h := PlatformHint("sfc", "SFC")
	if h == "" || !strings.Contains(h, "SNES") {
		t.Fatal(h)
	}
}
