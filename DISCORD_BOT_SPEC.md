# [디스코드 봇 개발용] TJ 신곡 추적 CLI (`tracking-tj`) 연동 명세서

TJ 노래방 신곡 추적 프로그램(`tracking-tj`)의 CLI 인터페이스 명세서입니다.
디스코드 봇 백엔드에서 자식 프로세스(`child_process.exec`, `subprocess.run` 등)로 호출하여 사용하시면 됩니다.

---

## 1. 별칭/번역명 후보 검색 (`search`)
사용자가 곡/아티스트를 등록하기 전, YouTube 및 나무위키에서 관련 별칭 및 번역명을 검색하여 JSON으로 반환합니다.
디스코드 봇에서 셀렉트 메뉴나 버튼(추천 별칭 선택)을 구성할 때 활용합니다.

- **명령어**:
  `./tracking-tj search "<검색어>"`

- **실행 예시**:
  `./tracking-tj search "処刑拍手"`

- **출력 (JSON stdout)**:
```json
{
  "query": "処刑拍手",
  "candidates": [
    {
      "title": "処刑拍手 (Execution Clap) / 重音テト",
      "source": "youtube",
      "alt_titles": [
        "Execution Clap",
        "처형박수",
        "쇼케이 하쿠슈"
      ],
      "description": "YouTube 검색 영상/음원 제목"
    },
    {
      "title": "처형박수",
      "source": "namuwiki",
      "alt_titles": [
        "処刑拍手",
        "Execution Clap"
      ],
      "description": "나무위키 문서: https://namu.wiki/w/..."
    }
  ]
}
```

---

## 2. 관심 곡 / 아티스트 추가 (`add`)
원제와 함께 선택된 별칭(Aliases)들을 한 번에 등록합니다.
별칭이 등록되면 노래방에 원제나 별칭 중 어느 것으로 올라와도 정상 매칭됩니다.

- **방법 1 (인수로 별칭 나열)**:
  `./tracking-tj add song "処刑拍手" "Execution Clap" "처형박수"`
  `./tracking-tj add artist "TRAP CHICK" "트랩칙"`

- **방법 2 (`--alias` 콤마 구분 옵션)**:
  `./tracking-tj add song "処刑拍手" --alias="Execution Clap,처형박수"`

- **방법 3 (별칭 자동 검색 후 일괄 등록 `--auto-search`)**:
  `./tracking-tj add song "処刑拍手" --auto-search`

- **출력 (stdout)**:
  `✅ 관심 곡 추가 완료: '処刑拍手' (별칭: Execution Clap, 처형박수)`

---

## 3. 관심 곡 / 아티스트 삭제 (`delete`)
원제를 삭제하면 DB에 매핑되어 있던 모든 관련 별칭들도 함께 자동 삭제됩니다.

- **명령어**:
  `./tracking-tj delete song "<곡제목>"`
  `./tracking-tj delete artist "<가수명>"`

- **실행 예시**:
  `./tracking-tj delete song "処刑拍手"`

- **출력 (stdout)**:
  `🗑️ 관심 곡이 삭제되었습니다: '処刑拍手'`

---

## 4. 현재 추적 목록 및 오늘 매칭 결과 조회 (`list --json`)
디스코드 임베드 메시지 출력을 위해 JSON 형태로 목록을 반환합니다.

- **명령어**:
  `./tracking-tj list --json`

- **출력 (JSON stdout)**:
```json
{
  "today_matches": [
    {
      "ID": 31,
      "Pro": 52691,
      "Title": "Execution Clap",
      "Artist": "TRAP CHICK(Feat.重音テト)",
      "PublishDate": "2026-09-01T00:00:00Z",
      "MatchedAt": "2026-09-01T11:38:25Z"
    }
  ],
  "artists": [
    {
      "title": "음율",
      "alt_titles": null,
      "start_from": "2026-07-29"
    }
  ],
  "songs": [
    {
      "title": "処刑拍手",
      "alt_titles": [
        "Execution Clap",
        "처형박수"
      ],
      "start_from": "2026-09-01"
    }
  ]
}
```

---

## 5. 신곡 매칭 자동 실행 모드 (알림 봇 / Cron용)
인수 없이 실행하면 TJ 신곡을 검사하며, **새로 매칭된 신곡이 있을 때만 TSV(탭 구분) 포맷으로 1줄씩 출력**합니다.
매칭이 없으면 아무것도 출력하지 않고 조용히 종료됩니다.

- **명령어**:
  `./tracking-tj`

- **신곡 매칭 시 출력 (TSV stdout)**:
```tsv
52691	Execution Clap	TRAP CHICK(Feat.重音テト)	2026-09-01
```
*(컬럼 순서: `곡번호[0]` \t `곡제목[1]` \t `가수명[2]` \t `수록일[3]`)*
