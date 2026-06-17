package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/gocolly/colly"
	"github.com/tmc/langchaingo/tools"
)

const (
	DefualtMaxDept   = 1
	DefualtParallels = 2
	DefualtDelay     = 3
	DefualtAsync     = true
	DefualtMaxPages  = 0
)

var ErrScrapingFailed = errors.New("scraper could not read URL, or scraping is not allowed for provided URL")

type Scraper struct {
	MaxDepth        int
	Parallels       int
	Delay           int64
	Blacklist       []string
	Async           bool
	MaxPages        int
	ContentSelector string // e.g. "article", "#mw-content-text", "main"
	UserAgent       string // HTTP User-Agent header; empty uses colly default
}

var _ tools.Tool = Scraper{}

// New creates a new instance of Scraper with the provided options.
func New(options ...Options) (*Scraper, error) {
	scraper := &Scraper{
		MaxDepth:  DefualtMaxDept,
		Parallels: DefualtParallels,
		Delay:     int64(DefualtDelay),
		Async:     DefualtAsync,
		MaxPages:  DefualtMaxPages,
		Blacklist: []string{
			"login",
			"signup",
			"signin",
			"register",
			"logout",
			"download",
			"redirect",
		},
	}

	for _, opt := range options {
		opt(scraper)
	}

	return scraper, nil
}

// Name returns the name of the Scraper.
func (s Scraper) Name() string {
	return "Web Scraper"
}

// Description returns the description of the Go function.
func (s Scraper) Description() string {
	return `Web Scraper will scan a URL and return the content of the web page as Markdown.
Useful for when you need to read the full text of a specific web page, including headings, lists, links, and tables.
Input should be a plain-text HTTP(S) URL string (not JSON), e.g. https://example.com/page.
Returns the page title, meta description, and the full page body converted to Markdown.`
}

// Call scrapes a website and returns the site data.
// The body of each page is converted to Markdown so that formatting
// (headings, lists, links, tables, code blocks) is preserved for LLM
// consumption. A CSS selector can be configured via WithContentSelector
// to extract only the main content area and skip navigation chrome.
func (s Scraper) Call(ctx context.Context, input string) (string, error) {
	targetURL := extractURL(input)
	_, err := url.ParseRequestURI(targetURL)
	if err != nil {
		return "", fmt.Errorf("%s: %w", ErrScrapingFailed, err)
	}

	c := colly.NewCollector(
		colly.MaxDepth(s.MaxDepth),
		colly.Async(s.Async),
	)
	if s.UserAgent != "" {
		c.UserAgent = s.UserAgent
	}

	err = c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: s.Parallels,
		Delay:       time.Duration(s.Delay) * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", ErrScrapingFailed, err)
	}

	var siteData strings.Builder
	var siteDataMutex sync.Mutex
	homePageLinks := make(map[string]bool)
	scrapedLinks := make(map[string]bool)
	scrapedLinksMutex := sync.RWMutex{}
	pageCount := 0
	pageCountMutex := sync.Mutex{}

	// Build a reusable markdown converter with table support.
	mdConverter := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(),
			table.NewTablePlugin(),
		),
	)

	c.OnRequest(func(r *colly.Request) {
		if ctx.Err() != nil {
			r.Abort()
			return
		}

		if s.MaxPages > 0 {
			pageCountMutex.Lock()
			if pageCount >= s.MaxPages {
				r.Abort()
				pageCountMutex.Unlock()
				return
			}
			pageCount++
			pageCountMutex.Unlock()
		}
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		currentURL := e.Request.URL.String()

		// Only process the page if it hasn't been visited yet.
		scrapedLinksMutex.Lock()
		if scrapedLinks[currentURL] {
			scrapedLinksMutex.Unlock()
			return
		}
		scrapedLinks[currentURL] = true
		scrapedLinksMutex.Unlock()

		var pageBuf strings.Builder
		pageBuf.WriteString("\n\nPage URL: " + currentURL)

		title := e.ChildText("title")
		if title != "" {
			pageBuf.WriteString("\nPage Title: " + title)
		}

		description := e.ChildAttr("meta[name=description]", "content")
		if description != "" {
			pageBuf.WriteString("\nPage Description: " + description)
		}

		// Extract main content HTML and convert to Markdown.
		contentHTML := extractContentHTML(e, s.ContentSelector)
		domain := e.Request.URL.Scheme + "://" + e.Request.URL.Host
		markdown, convErr := mdConverter.ConvertString(contentHTML, converter.WithDomain(domain))
		if convErr == nil && strings.TrimSpace(markdown) != "" {
			pageBuf.WriteString("\n\nPage Content (Markdown):\n" + markdown)
		} else {
			// Fallback: plain-text extraction for legacy compatibility.
			pageBuf.WriteString("\nHeaders:")
			e.ForEach("h1, h2, h3, h4, h5, h6", func(_ int, el *colly.HTMLElement) {
				pageBuf.WriteString("\n" + el.Text)
			})
			pageBuf.WriteString("\nContent:")
			e.ForEach("p", func(_ int, el *colly.HTMLElement) {
				pageBuf.WriteString("\n" + el.Text)
			})
		}

		if currentURL == targetURL {
			e.ForEach("a", func(_ int, el *colly.HTMLElement) {
				link := el.Attr("href")
				if link != "" && !homePageLinks[link] {
					homePageLinks[link] = true
					pageBuf.WriteString("\nLink: " + link)
				}
			})
		}

		siteDataMutex.Lock()
		siteData.WriteString(pageBuf.String())
		siteDataMutex.Unlock()
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteLink := e.Request.AbsoluteURL(link)

		u, err := url.Parse(absoluteLink)
		if err != nil {
			return
		}

		if u.Hostname() != e.Request.URL.Hostname() {
			return
		}

		for _, item := range s.Blacklist {
			if strings.Contains(u.Path, item) {
				return
			}
		}

		if u.Path == "/index.html" || u.Path == "" {
			u.Path = "/"
		}

		scrapedLinksMutex.RLock()
		if !scrapedLinks[u.String()] {
			scrapedLinksMutex.RUnlock()
			err := c.Visit(u.String())
			if err != nil {
				siteDataMutex.Lock()
				siteData.WriteString(fmt.Sprintf("\nError following link %s: %v", link, err))
				siteDataMutex.Unlock()
			}
		} else {
			scrapedLinksMutex.RUnlock()
		}
	})

	err = c.Visit(targetURL)
	if err != nil {
		return "", fmt.Errorf("%s: %w", ErrScrapingFailed, err)
	}

	// Wait for scraping to complete with context cancellation support.
	done := make(chan struct{})
	go func() {
		c.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
		// Scraping completed normally.
	}

	// Append all scraped links.
	siteData.WriteString("\n\nScraped Links:")
	for link := range scrapedLinks {
		siteData.WriteString("\n" + link)
	}

	return siteData.String(), nil
}

// extractContentHTML returns the inner HTML of the first node matching the
// given CSS selector. If selector is empty or no node matches, the inner HTML
// of <body> is returned as a fallback. If <body> is also absent, the inner
// HTML of the current element is returned.
func extractContentHTML(e *colly.HTMLElement, selector string) string {
	if selector != "" {
		if sel := e.DOM.Find(selector); sel.Length() > 0 {
			html, err := sel.First().Html()
			if err == nil && strings.TrimSpace(html) != "" {
				return html
			}
		}
	}
	if body := e.DOM.Find("body"); body.Length() > 0 {
		html, _ := body.First().Html()
		return html
	}
	html, _ := e.DOM.Html()
	return html
}

// extractURL handles both plain-text URLs and JSON-wrapped URLs. When the
// scraper is used alongside MCP tools (which accept JSON input), the LLM often
// formats every tool's input as JSON. This function unwraps {"url":"..."} and
// similar shapes so the scraper always receives a clean URL string.
func extractURL(input string) string {
	input = strings.TrimSpace(input)

	// Fast path: not JSON, return as-is.
	if !strings.HasPrefix(input, "{") {
		return input
	}

	// Try common JSON shapes: {"url": "..."}, {"URL": "..."}.
	var obj map[string]string
	if err := json.Unmarshal([]byte(input), &obj); err == nil {
		for _, key := range []string{"url", "URL", "Url"} {
			if v, ok := obj[key]; ok && v != "" {
				return strings.TrimSpace(v)
			}
		}
	}

	// Fallback: try to find a URL-looking substring inside the JSON.
	for _, scheme := range []string{"https://", "http://"} {
		if idx := strings.Index(input, scheme); idx != -1 {
			rest := input[idx:]
			// Cut at the next quote, space, or brace.
			for _, delim := range []string{"\"", " ", "}", ","} {
				if cut := strings.Index(rest, delim); cut != -1 {
					return rest[:cut]
				}
			}
			return rest
		}
	}

	return input
}
