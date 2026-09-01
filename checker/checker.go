package checker

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"tracking-tj/db"
	"tracking-tj/tjapi"
)

type MatchResult struct {
	Song              tjapi.TJSongItem
	MatchedType       string // "artist", "song", "both"
	MatchedReason     string
	MatchedSongTitles []string
}

type Checker struct {
	dictMap map[string][]string // lower string -> list of translated lower strings
}

func NewChecker(dictEntries []db.KoreanDictEntry) *Checker {
	dictMap := make(map[string][]string)

	for _, entry := range dictEntries {
		jpLower := strings.ToLower(strings.TrimSpace(entry.Japanese))
		krLower := strings.ToLower(strings.TrimSpace(entry.Korean))

		if jpLower != "" && krLower != "" {
			dictMap[jpLower] = append(dictMap[jpLower], krLower)
			dictMap[krLower] = append(dictMap[krLower], jpLower)
		}
	}

	return &Checker{
		dictMap: dictMap,
	}
}

var tokenDelimRegex = regexp.MustCompile(`[\(\)\[\]\{\}\,\/\&\+\-\|\;\:\~]|\b(?:feat|featuring|with|prod|by|vs|of)\b\.?`)

func extractTokens(text string) []string {
	parts := tokenDelimRegex.Split(text, -1)
	var tokens []string
	seen := make(map[string]bool)

	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			tokens = append(tokens, trimmed)
		}
		// Also split by whitespace
		words := strings.Fields(trimmed)
		if len(words) > 1 {
			for _, w := range words {
				if len(w) > 0 && !seen[w] {
					seen[w] = true
					tokens = append(tokens, w)
				}
			}
		}
	}
	return tokens
}

func (c *Checker) CheckMatches(
	newSongs []tjapi.TJSongItem,
	artists []db.TrackingItem,
	songs []db.TrackingItem,
	alreadyMatchedProMap map[int]bool,
	artistAltMap map[string][]string,
	songAltMap map[string][]string,
) []MatchResult {
	var results []MatchResult

	for _, item := range newSongs {
		// 이미 matched_history에 기록된 곡은 중복 알림 방지를 위해 스킵
		if alreadyMatchedProMap != nil && alreadyMatchedProMap[item.Pro] {
			continue
		}

		var matchedReasons []string
		var matchedSongTitles []string
		matchedArtist := false
		matchedSongTitle := false

		// 1. Artist matching (원제 및 별칭 검사)
		for _, artist := range artists {
			if c.isMatch(item.IndexSong, artist.Title, true) {
				matchedArtist = true
				matchedReasons = append(matchedReasons, fmt.Sprintf("가수 매칭: '%s'", artist.Title))
			} else if artistAltMap != nil {
				for _, alt := range artistAltMap[artist.Title] {
					if c.isMatch(item.IndexSong, alt, true) {
						matchedArtist = true
						matchedReasons = append(matchedReasons, fmt.Sprintf("가수 매칭 (별칭 '%s'): '%s'", alt, artist.Title))
						break
					}
				}
			}
		}

		// 2. Song title matching (원제 및 별칭 검사)
		for _, songTrack := range songs {
			if c.isMatch(item.IndexTitle, songTrack.Title, false) {
				matchedSongTitle = true
				matchedSongTitles = append(matchedSongTitles, songTrack.Title)
				matchedReasons = append(matchedReasons, fmt.Sprintf("곡 제목 매칭: '%s'", songTrack.Title))
			} else if songAltMap != nil {
				for _, alt := range songAltMap[songTrack.Title] {
					if c.isMatch(item.IndexTitle, alt, false) {
						matchedSongTitle = true
						matchedSongTitles = append(matchedSongTitles, songTrack.Title)
						matchedReasons = append(matchedReasons, fmt.Sprintf("곡 제목 매칭 (별칭 '%s'): '%s'", alt, songTrack.Title))
						break
					}
				}
			}
		}

		if matchedArtist || matchedSongTitle {
			matchedType := "artist"
			if matchedArtist && matchedSongTitle {
				matchedType = "both"
			} else if matchedSongTitle {
				matchedType = "song"
			}

			results = append(results, MatchResult{
				Song:              item,
				MatchedType:       matchedType,
				MatchedReason:     strings.Join(matchedReasons, " / "),
				MatchedSongTitles: matchedSongTitles,
			})
		}
	}

	return results
}

func (c *Checker) isMatch(target string, query string, isArtist bool) bool {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	queryLower := strings.ToLower(strings.TrimSpace(query))

	if queryLower == "" || targetLower == "" {
		return false
	}

	// 1. Direct exact match
	if targetLower == queryLower {
		return true
	}

	// 2. Token boundary match (delimiters like (), [], Feat., commas, etc.)
	tokens := extractTokens(targetLower)
	for _, tok := range tokens {
		if tok == queryLower {
			return true
		}
	}

	// 3. Dictionary translation lookup match
	if translations, ok := c.dictMap[queryLower]; ok {
		for _, trans := range translations {
			if targetLower == trans {
				return true
			}
			for _, tok := range tokens {
				if tok == trans {
					return true
				}
			}
		}
	}

	// 4. For Song Titles: check prefix/suffix match with boundary (e.g. "Flower (Acoustic Ver.)" matching "Flower")
	if !isArtist {
		if strings.HasPrefix(targetLower, queryLower) {
			rem := strings.TrimPrefix(targetLower, queryLower)
			if rem == "" || strings.HasPrefix(rem, " ") || strings.HasPrefix(rem, "(") || strings.HasPrefix(rem, "[") || strings.HasPrefix(rem, "-") {
				return true
			}
		}
	}

	return false
}

// Compare YYYY-MM-DD dates without time component
func isBeforeDate(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()

	if y1 < y2 {
		return true
	}
	if y1 > y2 {
		return false
	}
	if m1 < m2 {
		return true
	}
	if m1 < m2 {
		return false
	}
	return d1 < d2
}

func isAfterDate(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()

	if y1 > y2 {
		return true
	}
	if y1 < y2 {
		return false
	}
	if m1 > m2 {
		return true
	}
	if m1 < m2 {
		return false
	}
	return d1 > d2
}
