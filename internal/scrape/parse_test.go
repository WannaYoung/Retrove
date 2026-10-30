package scrape

import (
	"strings"
	"testing"
)

func TestParseJeuInfos(t *testing.T) {
	raw := []byte(`{
	  "header": {},
	  "response": {
	    "jeu": {
	      "id": "1234",
	      "nom": "Fallback",
	      "noms": [
	        {"region": "ss", "text": "Sonic"},
	        {"region": "us", "text": "Sonic the Hedgehog"},
	        {"region": "jp", "text": "ソニック"},
	        {"region": "cn", "text": "刺猬索尼克"}
	      ],
	      "developpeur": {"id": "1", "text": "Sega"},
	      "synopsis": [
	        {"langue": "en", "text": "Jump and run."},
	        {"langue": "zh", "text": "跑跳冒险。"}
	      ],
	      "dates": [{"region": "us", "text": "1991-06-23"}],
	      "medias": [
	        {"type": "box-2D", "url": "https://example.com/box.png", "region": "wor", "format": "png"},
	        {"type": "ss", "url": "https://example.com/ss.png", "region": "wor", "format": "png"}
	      ]
	    }
	  }
	}`)
	info, err := parseJeuInfos(raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "1234" {
		t.Fatalf("id %s", info.ID)
	}
	if info.TitleEN != "Sonic the Hedgehog" {
		t.Fatalf("title_en %q", info.TitleEN)
	}
	if info.TitleJA == "" || info.TitleZH == "" {
		t.Fatalf("titles ja=%q zh=%q", info.TitleJA, info.TitleZH)
	}
	if info.Developer != "Sega" {
		t.Fatalf("dev %q", info.Developer)
	}
	if info.Description != "跑跳冒险。" {
		t.Fatalf("desc %q", info.Description)
	}
	if info.ReleaseYear != 1991 {
		t.Fatalf("year %d", info.ReleaseYear)
	}
	box, ok := PreferMedia(info.Medias, "box")
	if !ok || box.URL == "" {
		t.Fatalf("box media %#v", box)
	}
}

func TestPreferMediaPrefersJPG(t *testing.T) {
	medias := []MediaRef{
		{Type: "box-2D", URL: "https://example.com/box.png", Region: "wor", Format: "png"},
		{Type: "box-2D", URL: "https://example.com/box.jpg", Region: "wor", Format: "jpg"},
	}
	box, ok := PreferMedia(medias, "box")
	if !ok || !strings.HasSuffix(box.URL, ".jpg") {
		t.Fatalf("want jpg, got %#v", box)
	}
	// better media type still wins over jpg of a worse type
	medias = append(medias, MediaRef{Type: "box-3D", URL: "https://example.com/box3d.jpg", Region: "wor", Format: "jpg"})
	box, ok = PreferMedia(medias, "box")
	if !ok || box.URL != "https://example.com/box.jpg" {
		t.Fatalf("type rank should beat format, got %#v", box)
	}
}

func TestParseNotFound(t *testing.T) {
	_, err := parseJeuInfos([]byte(`{"header":{},"response":{}}`))
	if AsKind(err) != KindNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestDetectQuotaError(t *testing.T) {
	err := detectAPIError([]byte(`{"header":{"erreur":"Votre quota de scrape est dépassé pour aujourd'hui !","code":430}}`))
	if AsKind(err) != KindQuota {
		t.Fatalf("got %v", err)
	}
}
