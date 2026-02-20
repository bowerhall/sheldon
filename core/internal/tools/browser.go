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

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/logger"
)

// BrowserConfig holds configuration for the browser tool
type BrowserConfig struct {
	UserAgent string
	Timeout   time.Duration
}

// DefaultBrowserConfig returns sensible defaults
func DefaultBrowserConfig() BrowserConfig {
	return BrowserConfig{
		UserAgent: "Sheldon/1.0 (AI Assistant; +https://github.com/bowerhall/sheldon)",
		Timeout:   30 * time.Second,
	}
}

// RegisterBrowserTools registers web browsing tools
func RegisterBrowserTools(registry *Registry, cfg BrowserConfig) {
	client := &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	fetchTool := llm.Tool{
		Name:        "fetch_url",
		Description: "Fetch content from a URL and extract readable text. Use this to read web pages, documentation, or any public URL. Returns extracted text content without HTML markup.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "The URL to fetch",
				},
				"extract": map[string]any{
					"type":        "string",
					"description": "What to extract: 'text' (default) for readable content, 'links' for all links, 'meta' for metadata",
					"enum":        []string{"text", "links", "meta"},
				},
			},
			"required": []string{"url"},
		},
	}

	registry.Register(fetchTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			URL     string `json:"url"`
			Extract string `json:"extract"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid params: %w", err)
		}

		if params.Extract == "" {
			params.Extract = "text"
		}

		// validate URL
		parsedURL, err := url.Parse(params.URL)
		if err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}

		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return "", fmt.Errorf("only http/https URLs supported")
		}

		logger.Debug("fetching url", "url", params.URL, "extract", params.Extract)

		req, err := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("User-Agent", cfg.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("fetch failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		// limit body size to 5MB
		body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
		if err != nil {
			return "", fmt.Errorf("read body: %w", err)
		}

		html := string(body)

		switch params.Extract {
		case "links":
			return extractLinks(html, parsedURL), nil
		case "meta":
			return extractMeta(html), nil
		default:
			return extractText(html), nil
		}
	})

	searchTool := llm.Tool{
		Name:        "search_web",
		Description: "Search the web using DuckDuckGo and return results. Use this to find information, documentation, or current events.",
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

		logger.Debug("searching web", "query", params.Query)

		// use DuckDuckGo HTML lite search
		searchURL := fmt.Sprintf("https://lite.duckduckgo.com/lite/?q=%s", url.QueryEscape(params.Query))

		req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("User-Agent", cfg.UserAgent)

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

	// truncate if too long (max ~10k chars for LLM context)
	if len(text) > 10000 {
		text = text[:10000] + "\n\n[Content truncated...]"
	}

	return text
}

// extractLinks returns all links from HTML
func extractLinks(html string, baseURL *url.URL) string {
	linkRe := regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']+)["'][^>]*>([^<]*)</a>`)
	matches := linkRe.FindAllStringSubmatch(html, -1)

	var links []string
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) < 3 {
			continue
		}

		href := strings.TrimSpace(m[1])
		text := strings.TrimSpace(m[2])

		// resolve relative URLs
		if !strings.HasPrefix(href, "http") {
			if parsed, err := baseURL.Parse(href); err == nil {
				href = parsed.String()
			}
		}

		// skip anchors and javascript
		if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			continue
		}

		if seen[href] {
			continue
		}
		seen[href] = true

		if text == "" {
			text = "[no text]"
		}

		links = append(links, fmt.Sprintf("- %s: %s", text, href))
	}

	if len(links) == 0 {
		return "No links found"
	}

	if len(links) > 50 {
		links = links[:50]
		links = append(links, "\n[More links truncated...]")
	}

	return strings.Join(links, "\n")
}

// extractMeta returns page metadata
func extractMeta(html string) string {
	var result strings.Builder

	// title
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	if m := titleRe.FindStringSubmatch(html); len(m) > 1 {
		result.WriteString(fmt.Sprintf("Title: %s\n", strings.TrimSpace(m[1])))
	}

	// meta tags
	metaRe := regexp.MustCompile(`(?i)<meta[^>]+(?:name|property)=["']([^"']+)["'][^>]+content=["']([^"']+)["']`)
	for _, m := range metaRe.FindAllStringSubmatch(html, -1) {
		if len(m) > 2 {
			name := strings.ToLower(m[1])
			if name == "description" || name == "og:description" || name == "og:title" ||
				name == "author" || name == "keywords" {
				result.WriteString(fmt.Sprintf("%s: %s\n", m[1], m[2]))
			}
		}
	}

	if result.Len() == 0 {
		return "No metadata found"
	}

	return result.String()
}

// extractSearchResults parses DuckDuckGo lite search results
func extractSearchResults(html string) string {
	var results []string

	// find result links from DuckDuckGo lite
	// pattern: <a rel="nofollow" href="..." class="result-link">title</a>
	linkRe := regexp.MustCompile(`(?is)<a[^>]+class="[^"]*result-link[^"]*"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	snippetRe := regexp.MustCompile(`(?is)<td[^>]*class="[^"]*result-snippet[^"]*"[^>]*>([^<]+)</td>`)

	linkMatches := linkRe.FindAllStringSubmatch(html, -1)
	snippetMatches := snippetRe.FindAllStringSubmatch(html, -1)

	for i, m := range linkMatches {
		if len(m) < 3 {
			continue
		}

		href := decodeHTMLEntities(strings.TrimSpace(m[1]))
		title := decodeHTMLEntities(strings.TrimSpace(m[2]))

		result := fmt.Sprintf("**%s**\n%s", title, href)

		if i < len(snippetMatches) && len(snippetMatches[i]) > 1 {
			snippet := decodeHTMLEntities(strings.TrimSpace(snippetMatches[i][1]))
			result += fmt.Sprintf("\n%s", snippet)
		}

		results = append(results, result)

		if len(results) >= 10 {
			break
		}
	}

	if len(results) == 0 {
		// fallback: try generic link extraction
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
