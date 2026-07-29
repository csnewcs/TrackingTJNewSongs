package main

import (
	"fmt"
	"log"
	"os"
	"time"

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

	case "add":
		if len(args) < 3 {
			printUsage()
			os.Exit(1)
		}
		subCmd := args[1]
		name := args[2]

		startDate := time.Now()
		if len(args) >= 4 {
			parsed, err := time.Parse("2006-01-02", args[3])
			if err != nil {
				log.Fatalf("❌ 날짜 형식이 올바르지 않습니다 (예: YYYY-MM-DD): %v", err)
			}
			startDate = parsed
		}

		if subCmd == "artist" {
			if err := database.AddTrackingArtist(name, startDate); err != nil {
				log.Fatalf("❌ 아티스트 추가 실패: %v", err)
			}
			fmt.Printf("✅ 관심 아티스트가 추가되었습니다: '%s' (시작일: %s)\n", name, startDate.Format("2006-01-02"))
		} else if subCmd == "song" {
			if err := database.AddTrackingSong(name, startDate); err != nil {
				log.Fatalf("❌ 관심 곡 추가 실패: %v", err)
			}
			fmt.Printf("✅ 관심 곡이 추가되었습니다: '%s' (시작일: %s)\n", name, startDate.Format("2006-01-02"))
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
		listTargets(database)

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
	searchYm := now.Format("200601")

	// 1. API 신곡 조회
	apiClient := tjapi.NewClient()
	songs, err := apiClient.FetchNewSongs(searchYm)
	if err != nil {
		log.Fatalf("❌ TJ API 수집 실패: %v", err)
	}

	// 2. DB 추적 정보 읽기
	artists, err := database.GetTrackingArtists()
	if err != nil {
		log.Fatalf("❌ 추적 아티스트 목록 조회 실패: %v", err)
	}

	trackingSongs, err := database.GetTrackingSongs()
	if err != nil {
		log.Fatalf("❌ 추적 곡 목록 조회 실패: %v", err)
	}

	dictEntries, err := database.GetKoreanDictionary()
	if err != nil {
		dictEntries = nil
	}

	// 3. 매칭 검사 수행
	chk := checker.NewChecker(dictEntries)
	matches := chk.CheckMatches(songs, artists, trackingSongs)

	// 4. last_updated 기록
	if err := database.RecordLastUpdated(now, len(matches)); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ last_updated DB 기록 실패: %v\n", err)
	}

	// 5. 매칭이 확인된 관심 곡(tracking_songs) 자동 삭제
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

	// 6. 매칭 결과가 있을 때만 최소 정보 출력
	if len(matches) > 0 {
		for _, m := range matches {
			fmt.Printf("곡번호: %d | 제목: %s | 가수: %s | 수록일: %s\n",
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

	logs, err := database.GetLastUpdatedLogs(5)
	if err != nil {
		log.Printf("⚠️ 실행 이력 조회 실패: %v", err)
	}

	fmt.Println("📌 [추적 중인 아티스트 목록]")
	if len(artists) == 0 {
		fmt.Println("   (등록된 아티스트가 없습니다)")
	} else {
		for _, a := range artists {
			fmt.Printf("   - %s (시작일: %s)\n", a.Title, a.StartFrom.Format("2006-01-02"))
		}
	}

	fmt.Println("\n📌 [추적 중인 곡 목록]")
	if len(songs) == 0 {
		fmt.Println("   (등록된 곡이 없습니다)")
	} else {
		for _, s := range songs {
			fmt.Printf("   - %s (시작일: %s)\n", s.Title, s.StartFrom.Format("2006-01-02"))
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

func printUsage() {
	fmt.Println(`사용법:
  ./tracking-tj                         : TJ 신곡 수집 및 매칭 검사 실행 (기본)
  ./tracking-tj run                     : TJ 신곡 수집 및 매칭 검사 실행
  ./tracking-tj add artist <가수명> [날짜] : 관심 아티스트 추가 (날짜 기본값: 오늘, 예: 2026-07-01)
  ./tracking-tj add song <곡제목> [날짜]   : 관심 곡 추가 (날짜 기본값: 오늘, 예: 2026-07-01)
  ./tracking-tj delete artist <가수명>   : 관심 아티스트 삭제
  ./tracking-tj delete song <곡제목>     : 관심 곡 삭제
  ./tracking-tj list                    : 추적 중인 대상 및 실행 이력 목록 조회`)
}
