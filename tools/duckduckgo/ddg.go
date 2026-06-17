package duckduckgo

import (
	"context"
	"errors"
	"net/http"

	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/tools/duckduckgo/internal"
)

// DefaultUserAgent defines a default value for user-agent header.
const DefaultUserAgent = "github.com/tmc/langchaingo/tools/duckduckgo"

// Tool defines a tool implementation for the DuckDuckGo Search.
type Tool struct {
	CallbacksHandler callbacks.Handler
	client           *internal.Client
}

var _ tools.Tool = Tool{}

// Option defines a function for configuring the DuckDuckGo tool.
type Option func(*Tool)

// WithHTTPClient sets a custom HTTP client for the DuckDuckGo tool.
func WithHTTPClient(client *http.Client) Option {
	return func(t *Tool) {
		t.client.SetHTTPClient(client)
	}
}

// New initializes a new DuckDuckGo Search tool with arguments for setting a
// max results per search query and a value for the user agent header.
func New(maxResults int, userAgent string, opts ...Option) (*Tool, error) {
	tool := &Tool{
		client: internal.New(maxResults, userAgent),
	}

	for _, opt := range opts {
		opt(tool)
	}

	return tool, nil
}

// Name returns a name for the tool.
func (t Tool) Name() string {
	return "DuckDuckGo Search"
}

// Description returns a description for the tool.
func (t Tool) Description() string {
	return `A wrapper around DuckDuckGo Search. A free search alternative to Google.
Useful for when you need to answer questions about current events or find specific web pages.
Input should be a plain-text search query string (not JSON), e.g. latest Go release notes.
Returns results as plain text, each formatted as:
  Title: <page title>
  Description: <short snippet>
  URL: <link>`
}

// Call performs the search and return the result.
func (t Tool) Call(ctx context.Context, input string) (string, error) {
	if t.CallbacksHandler != nil {
		t.CallbacksHandler.HandleToolStart(ctx, input)
	}

	result, err := t.client.Search(ctx, input)
	if err != nil {
		if errors.Is(err, internal.ErrNoGoodResult) {
			return "No good DuckDuckGo Search Results was found", nil
		}
		if t.CallbacksHandler != nil {
			t.CallbacksHandler.HandleToolError(ctx, err)
		}
		return "", err
	}

	if t.CallbacksHandler != nil {
		t.CallbacksHandler.HandleToolEnd(ctx, result)
	}

	return result, nil
}
