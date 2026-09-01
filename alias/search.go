package alias

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SearchResult struct {
	Query      string      `json:"query"`
	Candidates []Candidate `json:"candidates"`
}

type Candidate struct {
	Title       string   `json:"title"`
	Artist      string   `json:"artist,omitempty"`
	Source      string   `json:"source"`
	AltTitles   []string `json:"alt_titles"`
	Description string   `json:"description,omitempty"`
}

var client = &http.Client{
	Timeout: 10 * time.Second,
}

// SearchAliases searches YouTube and Namuwiki for localized and alternative titles
func SearchAliases(query string) (*SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	result := &SearchResult{
		Query:      query,
		Candidates: []Candidate{},
	}

	altSet := make(map[string]bool)

	// 1. YouTube Search
	ytCandidates := searchYouTube(query)
	for _, c := range ytCandidates {
		result.Candidates = append(result.Candidates, c)
		for _, alt := range c.AltTitles {
			altSet[strings.ToLower(alt)] = true
		}
	}

	// 2. Namuwiki Search & Redirects
	namuCandidates := searchNamuwiki(query)
	for _, c := range namuCandidates {
		result.Candidates = append(result.Candidates, c)
		for _, alt := range c.AltTitles {
			altSet[strings.ToLower(alt)] = true
		}
	}

	return result, nil
}

func searchYouTube(query string) []Candidate {
	var candidates []Candidate
	searchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", url.QueryEscape(query))
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,ja;q=0.8,en;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	bodyStr := string(body)

	// Extract video titles from ytInitialData
	titleRegex := regexp.MustCompile(`"title":\{"runs":\[\{"text":"([^"]+)"`)
	matches := titleRegex.FindAllStringSubmatch(bodyStr, 10)

	seenTitles := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		rawTitle := strings.TrimSpace(m[1])
		if rawTitle == "" || seenTitles[rawTitle] {
			continue
		}
		seenTitles[rawTitle] = true

		alts := extractSubTitles(rawTitle)
		candidates = append(candidates, Candidate{
			Title:       rawTitle,
			Source:      "youtube",
			AltTitles:   alts,
			Description: "YouTube 검색 영상/음원 제목",
		})
		if len(candidates) >= 3 {
			break
		}
	}

	return candidates
}

func searchNamuwiki(query string) []Candidate {
	var candidates []Candidate
	docURL := fmt.Sprintf("https://namu.wiki/w/%s", url.PathEscape(query))
	req, err := http.NewRequest("GET", docURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,ja;q=0.8,en;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		// Parse title tag or og:title
		ogTitleRegex := regexp.MustCompile(`<meta property="og:title" content="([^"]+)"`)
		m := ogTitleRegex.FindStringSubmatch(bodyStr)
		if len(m) >= 2 {
			docTitle := strings.TrimSuffix(m[1], " - 나무위키")
			docTitle = strings.TrimSpace(docTitle)

			alts := extractSubTitles(docTitle)
			// Check if query itself has different capitalization/scripts
			if !strings.EqualFold(docTitle, query) {
				alts = append(alts, docTitle)
			}

			candidates = append(candidates, Candidate{
				Title:       docTitle,
				Source:      "namuwiki",
				AltTitles:   uniqueStrings(alts),
				Description: fmt.Sprintf("나무위키 문서: https://namu.wiki/w/%s", url.PathEscape(docTitle)),
			})
		}
	}

	return candidates
}

func extractSubTitles(text string) []string {
	var alts []string
	seen := make(map[string]bool)

	add := func(s string) {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, `"'“”‘’`)
		if len(s) >= 2 && !seen[strings.ToLower(s)] {
			seen[strings.ToLower(s)] = true
			alts = append(alts, s)
		}
	}

	// 1. Parenthesized text: Foo (Bar) -> Bar, Foo
	parenRegex := regexp.MustCompile(`[\(\[\{]([^\)\]\}]+)[\)\]\}]`)
	for _, p := range parenRegex.FindAllStringSubmatch(text, -1) {
		if len(p) >= 2 {
			clean := strings.TrimSpace(p[1])
			if !strings.HasPrefix(strings.ToLower(clean), "feat") &&
				!strings.HasPrefix(strings.ToLower(clean), "prod") &&
				!strings.HasPrefix(strings.ToLower(clean), "mv") &&
				!strings.HasPrefix(strings.ToLower(clean), "official") {
				add(clean)
			}
		}
	}

	// 2. Dash / Slash / Colon splits: Artist - Title -> Title
	for _, sep := range []string{" - ", " / ", " | ", " : "} {
		if strings.Contains(text, sep) {
			parts := strings.Split(text, sep)
			for _, part := range parts {
				// Remove feat. xxx
				clean := regexp.MustCompile(`(?i)\(?(?:feat|ft|with|prod)\.?\s+[^\)]+\)?`).ReplaceAllString(part, "")
				add(clean)
			}
		}
	}

	return alts
}

func uniqueStrings(slice []string) []string {
	var res []string
	seen := make(map[string]bool)
	for _, s := range slice {
		s = strings.TrimSpace(s)
		if s != "" && !seen[strings.ToLower(s)] {
			seen[strings.ToLower(s)] = true
			res = append(res, s)
		}
	}
	return res
}

func ToJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}
