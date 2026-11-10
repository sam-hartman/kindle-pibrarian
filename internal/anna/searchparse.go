package anna

import (
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Anna's Archive search results (markup as of November 2026) are rows like:
//
//	<div class="flex pt-3 pb-3 border-b ...">
//	  <a href="/md5/<hash>"> cover </a>
//	  <div>
//	    <div class="... font-mono">lgli/Some/File/Path.epub</div>
//	    <a href="/md5/<hash>" class="js-vim-focus ...">Title</a>
//	    <a href="/search?q=..."><span class="icon-[mdi--user-edit]"></span> Author</a>
//	    <a href="/search?q=..."><span class="icon-[mdi--company]"></span> Publisher, City, Year</a>
//	    ...
//	    <div class="... font-semibold text-sm ...">English [en] · EPUB · 1.5MB · 2009 · Book (fiction) · ...</div>
//
// The page also has a "Recent downloads" ticker of unrelated /md5/ links,
// which must never be treated as results.

var (
	metaLangRe   = regexp.MustCompile(`^(.+?)\s*\[[A-Za-z]{2,3}(?:-[A-Za-z0-9]+)?\]$`)
	metaSizeRe   = regexp.MustCompile(`(?i)^\d+(?:\.\d+)?\s*(?:B|KB|MB|GB|TB)$`)
	metaFormatRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{1,4}$`)
)

// isRecentDownloadsLink reports whether a link sits in the "Recent downloads"
// ticker rather than in the search results.
func isRecentDownloadsLink(link *goquery.Selection) bool {
	return link.Closest(".js-recent-downloads-container").Length() > 0
}

// parseResultRow parses a search result in Anna's current row markup. It
// returns ok=false when the link is not inside such a row, so the caller can
// fall back to the older heuristics.
func parseResultRow(link *goquery.Selection) (b *Book, ok bool) {
	row := link.Closest("div.border-b")
	if row.Length() == 0 {
		return nil, false
	}
	titleLink := row.Find("a.js-vim-focus[href^='/md5/']").First()
	if titleLink.Length() == 0 {
		return nil, false
	}
	href, _ := titleLink.Attr("href")

	b = &Book{
		Title:     squashSpace(titleLink.Text()),
		Authors:   iconLinkText(row, "mdi--user-edit"),
		Publisher: iconLinkText(row, "mdi--company"),
		Hash:      strings.TrimPrefix(href, "/md5/"),
	}

	// The metadata line is the leaf div whose text is "·"-separated.
	row.Find("div").EachWithBreak(func(_ int, d *goquery.Selection) bool {
		if d.Find("div").Length() > 0 {
			return true
		}
		text := d.Text()
		if strings.Count(text, "·") < 2 {
			return true
		}
		b.Language, b.Format, b.Size = parseMetaLine(text)
		return b.Format == "" && b.Size == ""
	})

	// Fall back to the file path's extension for the format.
	if b.Format == "" {
		path := strings.ToLower(strings.TrimSpace(row.Find("div.font-mono").First().Text()))
		for _, f := range supportedFormats {
			if strings.HasSuffix(path, "."+f) {
				b.Format = f
				break
			}
		}
	}
	return b, true
}

// iconLinkText returns the text of the first link in row tagged with the given
// Material Design icon class (mdi--user-edit = author, mdi--company = publisher).
func iconLinkText(row *goquery.Selection, icon string) string {
	sel := row.Find("a").FilterFunction(func(_ int, a *goquery.Selection) bool {
		return a.Find("span[class*='"+icon+"']").Length() > 0
	}).First()
	return squashSpace(sel.Text())
}

// parseMetaLine parses "English [en] · Dutch [nl] · EPUB · 1.5MB · 2009 · ...".
// Multiple languages are joined with ", ". Format is lowercased.
func parseMetaLine(text string) (language, format, size string) {
	var langs []string
	for _, part := range strings.Split(text, "·") {
		part = strings.TrimSpace(strings.Trim(strings.ReplaceAll(part, " ", " "), " ✅"))
		switch {
		case part == "":
		case metaLangRe.MatchString(part):
			name := metaLangRe.FindStringSubmatch(part)[1]
			if mapped, ok := languageMap[strings.ToLower(name)]; ok {
				name = mapped
			}
			langs = append(langs, name)
		case size == "" && metaSizeRe.MatchString(part):
			size = part
		case format == "" && size == "" && metaFormatRe.MatchString(part):
			// Format precedes size on the line; checking size == "" keeps
			// later tokens from being mistaken for a format.
			format = strings.ToLower(part)
		}
	}
	return strings.Join(langs, ", "), format, size
}

func squashSpace(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, " ", " ")), " ")
}
