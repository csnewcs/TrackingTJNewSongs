#!/usr/bin/env python3
import datetime
import json
import os
import sys
import urllib.parse
import urllib.request

BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STATE_FILE = os.path.join(BASE_DIR, "scripts", "monitor_state.json")
LOG_FILE = os.path.join(BASE_DIR, "update_time_monitor.log")

API_URL = "https://www.tjmedia.com/legacy/api/newSongOfMonth"


def fetch_new_songs(search_ym: str):
    data = urllib.parse.urlencode({"searchYm": search_ym}).encode("utf-8")
    req = urllib.request.Request(
        API_URL,
        data=data,
        headers={"User-Agent": "TJ-Update-Monitor/1.0", "Content-Type": "application/x-www-form-urlencoded"},
    )
    with urllib.request.urlopen(req, timeout=15) as resp:
        if resp.status != 200:
            raise Exception(f"HTTP Status {resp.status}")
        res_json = json.loads(resp.read().decode("utf-8"))
        if res_json.get("resultCode") != "99":
            raise Exception(f"API Error: {res_json.get('resultMsg')}")
        result_data = res_json.get("resultData", {})
        return result_data.get("itemsTotalCount", 0), result_data.get("items", [])


def load_state():
    if os.path.exists(STATE_FILE):
        try:
            with open(STATE_FILE, "r", encoding="utf-8") as f:
                return json.load(f)
        except Exception:
            pass
    return {"last_count": 0, "known_pros": [], "last_check": ""}


def save_state(state):
    os.makedirs(os.path.dirname(STATE_FILE), exist_ok=True)
    with open(STATE_FILE, "w", encoding="utf-8") as f:
        json.dump(state, f, ensure_ascii=False, indent=2)


def log_message(msg: str):
    print(msg, flush=True)
    with open(LOG_FILE, "a", encoding="utf-8") as f:
        f.write(msg + "\n")


def main():
    now = datetime.datetime.now()
    now_str = now.strftime("%Y-%m-%d %H:%M:%S")
    search_ym = now.strftime("%Y%m")

    try:
        total_count, items = fetch_new_songs(search_ym)
    except Exception as e:
        log_message(f"[{now_str}] ❌ API 조회 실패 ({search_ym}): {e}")
        sys.exit(1)

    state = load_state()
    known_pros = set(state.get("known_pros", []))
    current_pros = {item["pro"] for item in items}

    # 신규 등록된 곡 탐지
    if known_pros:
        new_pros = current_pros - known_pros
    else:
        # 최초 실행 시에는 현재 목록을 초기 기준 상태로 저장
        new_pros = set()

    if new_pros:
        new_items = [item for item in items if item["pro"] in new_pros]
        log_message("=" * 80)
        log_message(f"[{now_str}] 🚨 [신곡 업데이트 감지] 신규 등록: 총 {len(new_items)}곡 (전체 {total_count}곡)")
        for idx, item in enumerate(new_items, 1):
            log_message(f"  [{idx}] 곡번호: {item['pro']} | 제목: {item['indexTitle']} | 가수: {item['indexSong']} | 수록일: {item.get('publishdate')}")
        log_message("=" * 80)
    else:
        if not known_pros:
            log_message(f"[{now_str}] 🚀 모니터링 초기 상태 등록 완료 (현재 총 {total_count}곡 감시 시작)")
        else:
            log_message(f"[{now_str}] ℹ️ 변동 없음 (총 {total_count}곡 유지)")

    # 상태 업데이트
    state["last_count"] = total_count
    state["known_pros"] = list(current_pros)
    state["last_check"] = now_str
    save_state(state)


if __name__ == "__main__":
    main()
