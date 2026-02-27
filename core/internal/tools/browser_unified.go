package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/browser"
	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
)

// BrowserConfig holds configuration for browser tools
type BrowserConfig struct {
	UserAgent string
	Timeout   time.Duration
}

// DefaultBrowserConfig returns sensible defaults
func DefaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Timeout:   30 * time.Second,
	}
}

// RegisterUnifiedBrowserTools registers browser tools that prefer sandbox, fallback to HTTP
func RegisterUnifiedBrowserTools(registry *Registry, runner *browser.Runner, httpCfg BrowserConfig) {
	client := &http.Client{
		Timeout: httpCfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// unified browse tool
	browseTool := llm.Tool{
		Name:        "browse",
		Description: "Open a URL and get page content. Uses a real browser for JavaScript rendering when available, falls back to HTTP fetch for static pages. Returns page structure with element references (@e1, @e2) for interaction.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to open (must start with http:// or https://)",
				},
			},
			"required": []string{"url"},
		},
	}

	registry.Register(browseTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		logger.Debug("browse tool", "url", params.URL)

		// try sandbox first if available
		if runner != nil {
			result, err := runner.Browse(ctx, params.URL)
			if err == nil {
				if len(result) > 15000 {
					result = result[:15000] + "\n\n[Content truncated...]"
				}
				return result, nil
			}
			logger.Debug("sandbox browse failed, falling back to HTTP", "error", err)
		}

		// fallback to HTTP fetch
		return httpFetch(ctx, client, httpCfg.UserAgent, params.URL)
	})

	// browse_click - only works with sandbox
	if runner != nil {
		clickTool := llm.Tool{
			Name:        "browse_click",
			Description: "Click an element on the page by its reference (e.g., @e1, @e2). Requires a previous browse call to get element references.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ref": map[string]any{
						"type":        "string",
						"description": "Element reference from snapshot (e.g., @e1, @e2)",
					},
				},
				"required": []string{"ref"},
			},
		}

		registry.Register(clickTool, func(ctx context.Context, args string) (string, error) {
			var params struct {
				Ref string `json:"ref"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("invalid params: %w", err)
			}

			logger.Debug("browse_click", "ref", params.Ref)

			result, err := runner.Click(ctx, params.Ref)
			if err != nil {
				return "", err
			}

			if len(result) > 15000 {
				result = result[:15000] + "\n\n[Content truncated...]"
			}
			return result, nil
		})

		// browse_fill
		fillTool := llm.Tool{
			Name:        "browse_fill",
			Description: "Fill a form field with text. Requires a previous browse call to get element references.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ref": map[string]any{
						"type":        "string",
						"description": "Element reference from snapshot (e.g., @e1, @e2)",
					},
					"value": map[string]any{
						"type":        "string",
						"description": "Text to fill into the field",
					},
				},
				"required": []string{"ref", "value"},
			},
		}

		registry.Register(fillTool, func(ctx context.Context, args string) (string, error) {
			var params struct {
				Ref   string `json:"ref"`
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("invalid params: %w", err)
			}

			logger.Debug("browse_fill", "ref", params.Ref)

			result, err := runner.Fill(ctx, params.Ref, params.Value)
			if err != nil {
				return "", err
			}

			if len(result) > 15000 {
				result = result[:15000] + "\n\n[Content truncated...]"
			}
			return result, nil
		})
	}

	// search_web - always HTTP (DuckDuckGo lite works fine)
	searchTool := llm.Tool{
		Name:        "search_web",
		Description: "Search the web using DuckDuckGo and return results.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{
					"type":        "string",
					"description": "Search query",
				},
			},
			"required": []string{"query"},
		},
	}

	registry.Register(searchTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		logger.Debug("search_web", "query", params.Query)

		// use HTML endpoint instead of lite (lite now requires CAPTCHA)
		searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(params.Query))

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("User-Agent", httpCfg.UserAgent)

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("search failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		if err != nil {
			return "", fmt.Errorf("read body: %w", err)
		}

		return extractSearchResults(string(body)), nil
	})
}

func httpFetch(ctx context.Context, client *http.Client, userAgent, targetURL string) (string, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("only http/https URLs supported")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	text := extractText(string(body))

	if len(text) > 15000 {
		text = text[:15000] + "\n\n[Content truncated...]"
	}

	return "[HTTP fallback - no JS rendering]\n\n" + text, nil
}

// NewBrowserRunner creates a browser runner if sandbox is available
func NewBrowserRunner(image string, timeout time.Duration) *browser.Runner {
	if image == "" {
		image = "sheldon-browser-sandbox:latest"
	}
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return browser.NewRunner(browser.Config{
		Image:   image,
		Timeout: timeout,
	})
}

// extractText removes HTML tags and extracts readable text
func extractText(html string) string {
	// remove script and style elements
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = scriptRe.ReplaceAllString(html, "")
	html = styleRe.ReplaceAllString(html, "")

	// remove HTML comments
	commentRe := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRe.ReplaceAllString(html, "")

	// convert common elements to text markers
	html = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</li>|</tr>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`(?i)<h[1-6][^>]*>`).ReplaceAllString(html, "\n\n## ")
	html = regexp.MustCompile(`(?i)</h[1-6]>`).ReplaceAllString(html, "\n")
	html = regexp.MustCompile(`(?i)<li[^>]*>`).ReplaceAllString(html, "\n- ")

	// remove remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, "")

	// decode HTML entities
	text = decodeHTMLEntities(text)

	// normalize whitespace
	text = regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	return text
}

// extractSearchResults parses DuckDuckGo HTML search results
func extractSearchResults(html string) string {
	var results []string

	// DDG HTML endpoint uses result__a class for links and result__snippet for snippets
	linkRe := regexp.MustCompile(`(?is)<a[^>]+class="result__a"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	snippetRe := regexp.MustCompile(`(?is)<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`)

	linkMatches := linkRe.FindAllStringSubmatch(html, -1)
	snippetMatches := snippetRe.FindAllStringSubmatch(html, -1)

	// helper to strip HTML tags from snippet
	stripTags := regexp.MustCompile(`<[^>]+>`)

	for i, m := range linkMatches {
		if len(m) < 3 {
			continue
		}

		href := decodeHTMLEntities(strings.TrimSpace(m[1]))
		title := decodeHTMLEntities(strings.TrimSpace(m[2]))

		// extract actual URL from DDG redirect link
		if strings.Contains(href, "uddg=") {
			if u, err := url.Parse(href); err == nil {
				if actual := u.Query().Get("uddg"); actual != "" {
					href = actual
				}
			}
		}

		result := fmt.Sprintf("**%s**\n%s", title, href)

		if i < len(snippetMatches) && len(snippetMatches[i]) > 1 {
			snippet := stripTags.ReplaceAllString(snippetMatches[i][1], "")
			snippet = decodeHTMLEntities(strings.TrimSpace(snippet))
			result += fmt.Sprintf("\n%s", snippet)
		}

		results = append(results, result)

		if len(results) >= 10 {
			break
		}
	}

	if len(results) == 0 {
		return "No results found. Try a different search query."
	}

	return strings.Join(results, "\n\n")
}

// decodeHTMLEntities converts common HTML entities to text
func decodeHTMLEntities(s string) string {
	replacements := map[string]string{
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&#39;":   "'",
		"&apos;":  "'",
		"&nbsp;":  " ",
		"&ndash;": "-",
		"&mdash;": "—",
		"&copy;":  "(c)",
		"&reg;":   "(R)",
		"&trade;": "(TM)",
	}

	for entity, char := range replacements {
		s = strings.ReplaceAll(s, entity, char)
	}

	// handle numeric entities
	numRe := regexp.MustCompile(`&#(\d+);`)
	s = numRe.ReplaceAllStringFunc(s, func(m string) string {
		var num int
		fmt.Sscanf(m, "&#%d;", &num)
		if num > 0 && num < 128 {
			return string(rune(num))
		}
		return m
	})

	return s
}
