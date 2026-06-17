package scraper

type Options func(*Scraper)

// WithMaxDepth sets the maximum depth for the Scraper.
//
// Default value: 1
func WithMaxDepth(maxDepth int) Options {
	return func(o *Scraper) {
		o.MaxDepth = maxDepth
	}
}

// WithParallelsNum sets the number of maximum allowed concurrent
// requests of the matching domains.
//
// Default value: 2
func WithParallelsNum(parallels int) Options {
	return func(o *Scraper) {
		o.Parallels = parallels
	}
}

// WithDelay creates an Options function that sets the delay of a Scraper.
//
// The delay parameter specifies the amount of time in milliseconds that
// the Scraper should wait between requests.
//
// Default value: 3
func WithDelay(delay int64) Options {
	return func(o *Scraper) {
		o.Delay = delay
	}
}

// WithAsync sets the async option for the Scraper.
//
// Default value: true
func WithAsync(async bool) Options {
	return func(o *Scraper) {
		o.Async = async
	}
}

// WithNewBlacklist creates an Options function that replaces
// the list of url endpoints to be excluded from the scraping,
// with a new list.
//
// Default value:
//
//	[]string{
//		"login",
//		"signup",
//		"signin",
//		"register",
//		"logout",
//		"download",
//		"redirect",
//	},
func WithNewBlacklist(blacklist []string) Options {
	return func(o *Scraper) {
		o.Blacklist = blacklist
	}
}

// WithBlacklist creates an Options function that appends
// the url endpoints to be excluded from the scraping,
// to the current list.
//
// Default value:
//
//	[]string{
//		"login",
//		"signup",
//		"signin",
//		"register",
//		"logout",
//		"download",
//		"redirect",
//	},
func WithBlacklist(blacklist []string) Options {
	return func(o *Scraper) {
		o.Blacklist = append(o.Blacklist, blacklist...)
	}
}

// WithMaxPages sets the maximum number of pages to scrape.
//
// Default value: 0 (no limit)
func WithMaxPages(maxPages int) Options {
	return func(o *Scraper) {
		o.MaxPages = maxPages
	}
}

// WithContentSelector sets a CSS selector used to extract the main content
// area from each page before converting to Markdown.
//
// When empty (the default) the converter uses the <body> element.
// Common values: "article", "main", "#mw-content-text", "#content".
func WithContentSelector(selector string) Options {
	return func(o *Scraper) {
		o.ContentSelector = selector
	}
}
