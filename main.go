package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"tracking-tj/alias"
	"tracking-tj/checker"
	"tracking-tj/db"
	"tracking-tj/tjapi"
)

func main() {
	cfg := LoadConfig()
	database, err := db.New(cfg.ConnString())
	if err != nil {
		log.Fatalf("❌ DB 연결 실패: %v", err)
	}
	defer database.Close()

	args := os.Args[1:]

	if len(args) == 0 {
		runTracker(database)
		return
	}

	command := args[0]

	switch command {
	case "run":
		runTracker(database)

	case "search":
		if len(args) < 2 {
			fmt.Println(`{"error": "search query required"}`)
			os.Exit(1)
		}
		query := strings.Join(args[1:], " ")
		res, err := alias.SearchAliases(query)
		if err != nil {
			fmt.Printf(`{"error": "%s"}`+"\n", err.Error())
			os.Exit(1)
		}
		fmt.Println(alias.ToJSON(res))

	case "add":
		if len(args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subCmd := args[1]
		name := args[2]

		var aliases []string
		autoSearch := false
		startDate := time.Now()

		for i := 3; i < len(args); i++ {
			arg := args[i]
			if arg == "--auto-search" || arg == "-s" {
				autoSearch = true
			} else if strings.HasPrefix(arg, "--alias=") {
				val := strings.TrimPrefix(arg, "--alias=")
				for _, a := range strings.Split(val, ",") {
					if t := strings.TrimSpace(a); t != "" {
						aliases = append(aliases, t)
					}
				}
			} else if arg == "--alias" && i+1 < len(args) {
				i++
				for _, a := range strings.Split(args[i], ",") {
					if t := strings.TrimSpace(a); t != "" {
						aliases = append(aliases, t)
					}
				}
			} else if parsed, err := time.Parse("2006-01-02", arg); err == nil {
				startDate = parsed
			} else {
				aliases = append(aliases, arg)
			}
		}

		// 자동 검색 옵션이 활성화된 경우 YouTube/나무위키에서 별칭 수집
		if autoSearch {
			if searchRes, err := alias.SearchAliases(name); err == nil {
				for _, c := range searchRes.Candidates {
					aliases = append(aliases, c.AltTitles...)
				}
			}
		}

		if subCmd == "artist" {
			if err := database.AddTrackingArtist(name, startDate); err != nil {
				log.Fatalf("❌ 아티스트 추가 실패: %v", err)
			}
			if len(aliases) > 0 {
				_ = database.AddAltTitles("artist", name, aliases, "manual")
			}
			fmt.Printf("✅ 관심 아티스트 추가 완료: '%s' (별칭: %s)\n", name, formatAliases(aliases))
		} else if subCmd == "song" {
			if err := database.AddTrackingSong(name, startDate); err != nil {
				log.Fatalf("❌ 관심 곡 추가 실패: %v", err)
			}
			if len(aliases) > 0 {
				_ = database.AddAltTitles("song", name, aliases, "manual")
			}
			fmt.Printf("✅ 관심 곡 추가 완료: '%s' (별칭: %s)\n", name, formatAliases(aliases))
		} else {
			printUsage()
			os.Exit(1)
		}

	case "delete", "remove", "del":
		if len(args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subCmd := args[1]
		name := args[2]

		if subCmd == "artist" {
			count, err := database.DeleteTrackingArtist(name)
			if err != nil {
				log.Fatalf("❌ 아티스트 삭제 실패: %v", err)
			}
			if count == 0 {
				fmt.Printf("⚠️ 삭제 대상 아티스트 '%s'를 찾을 수 없습니다.\n", name)
			} else {
				fmt.Printf("🗑️ 관심 아티스트가 삭제되었습니다: '%s'\n", name)
			}
		} else if subCmd == "song" {
			count, err := database.DeleteTrackingSong(name)
			if err != nil {
				log.Fatalf("❌ 곡 삭제 실패: %v", err)
			}
			if count == 0 {
				fmt.Printf("⚠️ 삭제 대상 곡 '%s'를 찾을 수 없습니다.\n", name)
			} else {
				fmt.Printf("🗑️ 관심 곡이 삭제되었습니다: '%s'\n", name)
			}
		} else {
			printUsage()
			os.Exit(1)
		}

	case "list":
		asJSON := false
		for _, a := range args[1:] {
			if a == "--json" || a == "-j" {
				asJSON = true
			}
		}
		if asJSON {
			listTargetsJSON(database)
		} else {
			listTargets(database)
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("알 수 없는 명령: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runTracker(database *db.DB) {
	now := time.Now()
	currentYm := now.Format("200601")
	prevYm := now.AddDate(0, -1, 0).Format("200601")

	// 1. API 신곡 조회 (월 전환기 누락 방지를 위해 당월 및 전월 신곡 수집)
	apiClient := tjapi.NewClient()
	songsCurrent, err := apiClient.FetchNewSongs(currentYm)
	if err != nil {
		log.Fatalf("❌ TJ API 당월 수집 실패: %v", err)
	}

	songsPrev, err := apiClient.FetchNewSongs(prevYm)
	if err != nil {
		songsPrev = nil
	}

	seen := make(map[int]bool)
	var songs []tjapi.TJSongItem
	for _, item := range append(songsCurrent, songsPrev...) {
		if !seen[item.Pro] {
			seen[item.Pro] = true
			songs = append(songs, item)
		}
	}

	// 2. DB 추적 정보 및 별칭(alt_titles) 읽기
	artists, err := database.GetTrackingArtists()
	if err != nil {
		log.Fatalf("❌ 추적 아티스트 목록 조회 실패: %v", err)
	}

	trackingSongs, err := database.GetTrackingSongs()
	if err != nil {
		log.Fatalf("❌ 추적 곡 목록 조회 실패: %v", err)
	}

	artistAltMap, songAltMap, _ := database.GetAllAltTitlesMap()

	dictEntries, err := database.GetKoreanDictionary()
	if err != nil {
		dictEntries = nil
	}

	// 3. 이미 알림/매칭이 완료된 곡 번호(pro) 목록 조회 (중복 알림 방지)
	alreadyMatchedProMap, err := database.GetMatchedHistoryProMap()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ 매칭 이력 조회 실패: %v\n", err)
	}

	// 4. 매칭 검사 수행 (신규 신곡 대상)
	chk := checker.NewChecker(dictEntries)
	matches := chk.CheckMatches(songs, artists, trackingSongs, alreadyMatchedProMap, artistAltMap, songAltMap)

	// 5. last_updated 기록
	if err := database.RecordLastUpdated(now, len(matches)); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ last_updated DB 기록 실패: %v\n", err)
	}

	// 6. 매칭 상세 내역 DB 저장 (matched_history)
	for _, m := range matches {
		pubDate, _ := time.Parse("2006-01-02", m.Song.PublishDate)
		if err := database.AddMatchedHistory(m.Song.Pro, m.Song.IndexTitle, m.Song.IndexSong, pubDate); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ matched_history 저장 실패: %v\n", err)
		}
	}

	// 7. 매칭이 확인된 관심 곡(tracking_songs) 자동 삭제
	deletedSongs := make(map[string]bool)
	for _, m := range matches {
		for _, songTitle := range m.MatchedSongTitles {
			if !deletedSongs[songTitle] {
				deletedSongs[songTitle] = true
				if _, err := database.DeleteTrackingSong(songTitle); err != nil {
					fmt.Fprintf(os.Stderr, "⚠️ 관심 곡 '%s' 삭제 실패: %v\n", songTitle, err)
				}
			}
		}
	}

	// 8. 매칭 결과가 있을 때 봇 파싱용 TSV(Tab-Separated Values) 형식 출력
	if len(matches) > 0 {
		for _, m := range matches {
			fmt.Printf("%d\t%s\t%s\t%s\n",
				m.Song.Pro, m.Song.IndexTitle, m.Song.IndexSong, m.Song.PublishDate)
		}
	}
}

func listTargets(database *db.DB) {
	artists, err := database.GetTrackingArtists()
	if err != nil {
		log.Fatalf("❌ 아티스트 목록 조회 실패: %v", err)
	}

	songs, err := database.GetTrackingSongs()
	if err != nil {
		log.Fatalf("❌ 곡 목록 조회 실패: %v", err)
	}

	artistAltMap, songAltMap, _ := database.GetAllAltTitlesMap()

	todayMatches, err := database.GetTodayMatchedHistory()
	if err != nil {
		log.Printf("⚠️ 오늘 매칭 히스토리 조회 실패: %v", err)
	}

	logs, err := database.GetLastUpdatedLogs(5)
	if err != nil {
		log.Printf("⚠️ 실행 이력 조회 실패: %v", err)
	}

	fmt.Println("🎉 [오늘 매칭/등록 확인된 신곡 목록]")
	if len(todayMatches) == 0 {
		fmt.Println("   (오늘 등록 확인된 신곡이 없습니다)")
	} else {
		for _, m := range todayMatches {
			fmt.Printf("   - [%d] %s - %s (수록일: %s, 확인시간: %s)\n",
				m.Pro, m.Title, m.Artist, m.PublishDate.Format("2006-01-02"), m.MatchedAt.Format("15:04:05"))
		}
	}

	fmt.Println("\n📌 [추적 중인 아티스트 목록]")
	if len(artists) == 0 {
		fmt.Println("   (등록된 아티스트가 없습니다)")
	} else {
		for _, a := range artists {
			alts := artistAltMap[a.Title]
			if len(alts) > 0 {
				fmt.Printf("   - %s (별칭: %s, 시작일: %s)\n", a.Title, strings.Join(alts, ", "), a.StartFrom.Format("2006-01-02"))
			} else {
				fmt.Printf("   - %s (시작일: %s)\n", a.Title, a.StartFrom.Format("2006-01-02"))
			}
		}
	}

	fmt.Println("\n📌 [추적 중인 곡 목록]")
	if len(songs) == 0 {
		fmt.Println("   (등록된 곡이 없습니다)")
	} else {
		for _, s := range songs {
			alts := songAltMap[s.Title]
			if len(alts) > 0 {
				fmt.Printf("   - %s (별칭: %s, 시작일: %s)\n", s.Title, strings.Join(alts, ", "), s.StartFrom.Format("2006-01-02"))
			} else {
				fmt.Printf("   - %s (시작일: %s)\n", s.Title, s.StartFrom.Format("2006-01-02"))
			}
		}
	}

	fmt.Println("\n📜 [최근 last_updated 실행 이력 (최대 5건)]")
	if len(logs) == 0 {
		fmt.Println("   (실행 이력이 없습니다)")
	} else {
		for _, l := range logs {
			fmt.Printf("   - %s : %d건 매칭\n", l.Date.Format("2006-01-02"), l.Matched)
		}
	}
}

type ListJSONOutput struct {
	TodayMatches []db.MatchedHistoryRecord `json:"today_matches"`
	Artists      []TargetWithAlts          `json:"artists"`
	Songs        []TargetWithAlts          `json:"songs"`
}

type TargetWithAlts struct {
	Title     string   `json:"title"`
	AltTitles []string `json:"alt_titles"`
	StartFrom string   `json:"start_from"`
}

func listTargetsJSON(database *db.DB) {
	artists, _ := database.GetTrackingArtists()
	songs, _ := database.GetTrackingSongs()
	artistAltMap, songAltMap, _ := database.GetAllAltTitlesMap()
	todayMatches, _ := database.GetTodayMatchedHistory()

	out := ListJSONOutput{
		TodayMatches: todayMatches,
		Artists:      []TargetWithAlts{},
		Songs:        []TargetWithAlts{},
	}

	for _, a := range artists {
		out.Artists = append(out.Artists, TargetWithAlts{
			Title:     a.Title,
			AltTitles: artistAltMap[a.Title],
			StartFrom: a.StartFrom.Format("2006-01-02"),
		})
	}

	for _, s := range songs {
		out.Songs = append(out.Songs, TargetWithAlts{
			Title:     s.Title,
			AltTitles: songAltMap[s.Title],
			StartFrom: s.StartFrom.Format("2006-01-02"),
		})
	}

	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

func formatAliases(aliases []string) string {
	if len(aliases) == 0 {
		return "(없음)"
	}
	return strings.Join(aliases, ", ")
}

func printUsage() {
	fmt.Println(`사용법:
  ./tracking-tj                                      : TJ 신곡 수집 및 매칭 검사 실행 (신규 매칭 시 TSV 출력)
  ./tracking-tj search <검색어>                       : YouTube/나무위키에서 별칭/번역명 검색 (JSON 출력)
  ./tracking-tj add song <곡제목> [별칭1 별칭2...]     : 관심 곡 및 별칭 추가
  ./tracking-tj add song <곡제목> --alias="별칭1,별칭2": 관심 곡 및 별칭 일괄 추가
  ./tracking-tj add song <곡제목> --auto-search      : 곡 추가 및 YouTube/나무위키 별칭 자동 수집 등록
  ./tracking-tj add artist <가수명> [별칭1 별칭2...]   : 관심 아티스트 및 별칭 추가
  ./tracking-tj delete song <곡제목>                  : 관심 곡 및 관련 별칭 일괄 삭제
  ./tracking-tj delete artist <가수명>                : 관심 아티스트 및 관련 별칭 일괄 삭제
  ./tracking-tj list [--json]                        : 추적 목록 및 실행 이력 조회 (텍스트 또는 JSON)`)
}
