package checker

import (
	"fmt"
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

func (c *Checker) CheckMatches(
	newSongs []tjapi.TJSongItem,
	artists []db.TrackingItem,
	songs []db.TrackingItem,
	lastUpdatedDate *time.Time,
) []MatchResult {
	var results []MatchResult

	for _, item := range newSongs {
		songPublishDate, err := time.Parse("2006-01-02", item.PublishDate)
		if err != nil {
			songPublishDate = time.Time{}
		}

		// last_updated 날짜가 존재할 경우, 해당 날짜 이후(publishdate > lastUpdatedDate)에 올라온 신곡만 검사
		if lastUpdatedDate != nil && !songPublishDate.IsZero() {
			if !isAfterDate(songPublishDate, *lastUpdatedDate) {
				continue
			}
		}

		var matchedReasons []string
		var matchedSongTitles []string
		matchedArtist := false
		matchedSongTitle := false

		// 1. Artist matching
		for _, artist := range artists {
			if c.isMatch(item.IndexSong, artist.Title) {
				matchedArtist = true
				matchedReasons = append(matchedReasons, fmt.Sprintf("가수 매칭: '%s'", artist.Title))
			}
		}

		// 2. Song title matching
		for _, songTrack := range songs {
			if c.isMatch(item.IndexTitle, songTrack.Title) {
				matchedSongTitle = true
				matchedSongTitles = append(matchedSongTitles, songTrack.Title)
				matchedReasons = append(matchedReasons, fmt.Sprintf("곡 제목 매칭: '%s'", songTrack.Title))
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

func (c *Checker) isMatch(target string, query string) bool {
	targetLower := strings.ToLower(strings.TrimSpace(target))
	queryLower := strings.ToLower(strings.TrimSpace(query))

	if queryLower == "" || targetLower == "" {
		return false
	}

	// Direct substring match
	if strings.Contains(targetLower, queryLower) || strings.Contains(queryLower, targetLower) {
		return true
	}

	// Dictionary translation lookup match
	if translations, ok := c.dictMap[queryLower]; ok {
		for _, trans := range translations {
			if strings.Contains(targetLower, trans) || strings.Contains(trans, targetLower) {
				return true
			}
		}
	}

	if translations, ok := c.dictMap[targetLower]; ok {
		for _, trans := range translations {
			if strings.Contains(queryLower, trans) || strings.Contains(trans, queryLower) {
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
