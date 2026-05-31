package anna

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"go.uber.org/zap"

	"github.com/sam-hartman/kindle-pibrarian/internal/logger"
)

// Anna's Archive rotates its domains frequently; a dead mirror returns a parked
// page (the old hardcoded annas-archive.li is currently parked, which silently
// breaks every search/download). We therefore keep an ordered list of mirrors
// and try them in turn, and allow the operator to override the list entirely via
// the ANNAS_BASE_URLS env var (comma-separated, highest priority first) so a
// future rotation is a config change, not a code change + redeploy.
var defaultAnnasBases = []string{
	"https://annas-archive.gl",
	"https://annas-archive.se",
	"https://annas-archive.org",
}

// annasBases returns the ordered list of Anna's Archive base URLs to try.
func annasBases() []string {
	if v := strings.TrimSpace(os.Getenv("ANNAS_BASE_URLS")); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimRight(strings.TrimSpace(p), "/")
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return defaultAnnasBases
}

func annasSearchURL(base, query string) string {
	return base + "/search?q=" + url.QueryEscape(query)
}

func annasDownloadURL(base, hash, key string, domainIndex int) string {
	return fmt.Sprintf("%s/dyn/api/fast_download.json?md5=%s&key=%s&domain_index=%d", base, hash, key, domainIndex)
}

// scrapeSearch queries Anna's Archive search across the configured mirrors and
// returns the md5 result links from the first mirror that yields any. A parked
// or rotated domain returns zero links, so the loop self-heals to a live mirror.
func scrapeSearch(query string) []*colly.HTMLElement {
	l := logger.GetLogger()
	for _, base := range annasBases() {
		results := scrapeSearchOnce(base, query)
		if len(results) > 0 {
			l.Info("Search mirror succeeded", zap.String("base", base), zap.Int("links", len(results)))
			return results
		}
		l.Warn("Search mirror returned no results; trying next mirror", zap.String("base", base))
	}
	return nil
}

func scrapeSearchOnce(base, query string) []*colly.HTMLElement {
	l := logger.GetLogger()

	c := colly.NewCollector(colly.Async(true))
	// Standard TLS verification (no InsecureSkipVerify) — a bad mirror should
	// fail and fall through to the next, not be silently trusted.
	c.SetClient(&http.Client{Timeout: 30 * time.Second})
	c.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

	bookList := make([]*colly.HTMLElement, 0)

	c.OnHTML("a[href^='/md5/']", func(e *colly.HTMLElement) {
		bookList = append(bookList, e)
	})
	c.OnHTML("[class*='book'], [class*='item'], [class*='result']", func(e *colly.HTMLElement) {
		link := e.DOM.Find("a[href^='/md5/']").First()
		if link.Length() > 0 {
			if href, _ := link.Attr("href"); href != "" {
				bookList = append(bookList, &colly.HTMLElement{DOM: link, Request: e.Request, Response: e.Response})
			}
		}
	})
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.5")
	})
	c.OnError(func(r *colly.Response, err error) {
		l.Warn("Search request failed", zap.String("base", base), zap.Error(err))
	})

	c.Visit(annasSearchURL(base, query))
	c.Wait()
	return bookList
}
