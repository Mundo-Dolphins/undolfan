package content

import (
	"strings"
	"testing"
	"time"

	"github.com/Mundo-Dolphins/undolfan/internal/bluesky"
)

func TestTitleAndDescriptionAreDeterministic(t *testing.T) {
	created := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	text := "1/ 🧵 Tua no está participando hoy en los ejercicios de equipo. Segunda frase con más detalle. https://example.com"
	title := TitleFromText(text, created, 110)
	if title != "Tua no está participando hoy en los ejercicios de equipo" {
		t.Fatalf("unexpected title %q", title)
	}
	desc := DescriptionFromText(text, 35)
	if strings.Contains(desc, "1/") || strings.Contains(desc, "https://example.com") {
		t.Fatalf("description was not cleaned: %q", desc)
	}
}

func TestMarkdownFromFacets(t *testing.T) {
	post := bluesky.Post{Record: bluesky.Record{
		Text: "Lee esto #Dolphins @undolfan",
		Facets: []bluesky.Facet{
			{Index: bluesky.ByteSlice{ByteStart: 0, ByteEnd: 3}, Features: []bluesky.FacetFeature{{Type: "app.bsky.richtext.facet#link", URI: "https://mundodolphins.es"}}},
			{Index: bluesky.ByteSlice{ByteStart: 9, ByteEnd: 18}, Features: []bluesky.FacetFeature{{Type: "app.bsky.richtext.facet#tag", Tag: "Dolphins"}}},
			{Index: bluesky.ByteSlice{ByteStart: 19, ByteEnd: 29}, Features: []bluesky.FacetFeature{{Type: "app.bsky.richtext.facet#mention", DID: "did:plc:author"}}},
		},
	}}
	md, tags := MarkdownFromPost(post)
	if !strings.Contains(md, "[Lee](https://mundodolphins.es)") {
		t.Fatalf("link facet not rendered: %q", md)
	}
	if len(tags) != 1 || tags[0] != "dolphins" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
}

func TestSlugIsSafe(t *testing.T) {
	slug := SlugFromText("../Túa entrena: día 1", "abc", 80)
	if slug != "tua-entrena-dia-1" {
		t.Fatalf("unexpected slug %q", slug)
	}
}
