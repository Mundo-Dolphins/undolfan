package content

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Mundo-Dolphins/undolfan/internal/bluesky"
)

var (
	threadPrefix = regexp.MustCompile(`(?i)^\s*(?:\d+\s*(?:/|\.)(?:\s*\d+)?|🧵)\s*`)
	finalURL     = regexp.MustCompile(`\s+https?://\S+\s*$`)
	spaceRun     = regexp.MustCompile(`[ \t]+`)
)

func CleanLead(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for {
		next := threadPrefix.ReplaceAllString(text, "")
		if next == text {
			break
		}
		text = next
	}
	text = strings.TrimSpace(text)
	text = finalURL.ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

func TitleFromText(text string, created TimeLike, max int) string {
	clean := CleanLead(text)
	if max <= 0 {
		max = 110
	}
	if clean == "" {
		return fallbackTitle(created)
	}
	for _, sep := range []string{". ", "? ", "! ", "\n"} {
		if i := strings.Index(clean, sep); i > 20 {
			clean = strings.TrimSpace(clean[:i+1])
			break
		}
	}
	clean = truncateWords(clean, max)
	if clean == "" {
		return fallbackTitle(created)
	}
	return strings.TrimRight(clean, ". ")
}

type TimeLike interface {
	Format(layout string) string
	IsZero() bool
}

func DescriptionFromText(text string, max int) string {
	clean := CleanLead(text)
	if max <= 0 {
		max = 170
	}
	return truncateWords(clean, max)
}

func MarkdownFromPost(post bluesky.Post) (string, []string) {
	text := strings.ReplaceAll(post.Record.Text, "\r\n", "\n")
	tags := map[string]bool{}
	type replacement struct {
		start int
		end   int
		text  string
	}
	var repls []replacement
	for _, facet := range post.Record.Facets {
		if facet.Index.ByteStart < 0 || facet.Index.ByteEnd > len(text) || facet.Index.ByteStart >= facet.Index.ByteEnd {
			continue
		}
		raw := text[facet.Index.ByteStart:facet.Index.ByteEnd]
		for _, feature := range facet.Features {
			switch feature.Type {
			case "app.bsky.richtext.facet#link":
				if feature.URI != "" {
					repls = append(repls, replacement{facet.Index.ByteStart, facet.Index.ByteEnd, fmt.Sprintf("[%s](%s)", escapeMarkdown(raw), safeURL(feature.URI))})
				}
			case "app.bsky.richtext.facet#mention":
				if feature.DID != "" {
					repls = append(repls, replacement{facet.Index.ByteStart, facet.Index.ByteEnd, escapeMarkdown(raw)})
				}
			case "app.bsky.richtext.facet#tag":
				tag := normalizeTag(feature.Tag)
				if tag != "" {
					tags[tag] = true
				}
				repls = append(repls, replacement{facet.Index.ByteStart, facet.Index.ByteEnd, escapeMarkdown(raw)})
			}
		}
	}
	sort.Slice(repls, func(i, j int) bool { return repls[i].start > repls[j].start })
	for _, r := range repls {
		text = text[:r.start] + r.text + text[r.end:]
	}
	text = cleanThreadMarkers(text)
	parts := strings.Split(text, "\n\n")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(spaceRun.ReplaceAllString(p, " "))
		if p != "" {
			out = append(out, p)
		}
	}
	var tagList []string
	for tag := range tags {
		tagList = append(tagList, tag)
	}
	sort.Strings(tagList)
	return strings.Join(out, "\n\n"), tagList
}

func cleanThreadMarkers(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = threadPrefix.ReplaceAllString(line, "")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func SlugFromText(text, rkey string, max int) string {
	clean := strings.ToLower(CleanLead(text))
	var b bytes.Buffer
	dash := false
	for _, r := range clean {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case strings.ContainsRune("áàäâ", r):
			b.WriteByte('a')
			dash = false
		case strings.ContainsRune("éèëê", r):
			b.WriteByte('e')
			dash = false
		case strings.ContainsRune("íìïî", r):
			b.WriteByte('i')
			dash = false
		case strings.ContainsRune("óòöô", r):
			b.WriteByte('o')
			dash = false
		case strings.ContainsRune("úùüû", r):
			b.WriteByte('u')
			dash = false
		case r == 'ñ':
			b.WriteByte('n')
			dash = false
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
		if b.Len() >= max {
			break
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "post-" + safeRKey(rkey)
	}
	return slug
}

func RKey(uri string) string {
	i := strings.LastIndex(uri, "/")
	if i == -1 {
		return uri
	}
	return uri[i+1:]
}

func BlueskyURL(handle, uri string) string {
	return "https://bsky.app/profile/" + handle + "/post/" + RKey(uri)
}

func truncateWords(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	cut := max
	for cut > max-30 && cut > 0 && !unicode.IsSpace(runes[cut-1]) {
		cut--
	}
	if cut <= 0 {
		cut = max
	}
	return strings.TrimSpace(string(runes[:cut]))
}

func fallbackTitle(t TimeLike) string {
	if t == nil || t.IsZero() {
		return "Publicacion de Bluesky"
	}
	return "Hilo del " + t.Format("2 de January de 2006")
}

func normalizeTag(tag string) string {
	tag = strings.Trim(strings.ToLower(tag), "# \t\r\n")
	if tag == "" {
		return ""
	}
	return tag
}

func safeURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "#"
	}
	return u.String()
}

func escapeMarkdown(s string) string {
	replacer := strings.NewReplacer("[", "\\[", "]", "\\]")
	return replacer.Replace(s)
}

func safeRKey(s string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(s, "")
}
