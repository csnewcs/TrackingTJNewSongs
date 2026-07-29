package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type TrackingItem struct {
	Title     string
	StartFrom time.Time
}

type KoreanDictEntry struct {
	Japanese string
	Korean   string
	StoredAt time.Time
}

type LastUpdated struct {
	Date    time.Time
	Matched int
}

type DB struct {
	db *sql.DB
}

func New(connStr string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db: sqlDB}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) GetTrackingArtists() ([]TrackingItem, error) {
	rows, err := d.db.Query("SELECT title, start_from FROM tracking_artists ORDER BY start_from DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query tracking_artists: %w", err)
	}
	defer rows.Close()

	var items []TrackingItem
	for rows.Next() {
		var item TrackingItem
		if err := rows.Scan(&item.Title, &item.StartFrom); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (d *DB) GetTrackingSongs() ([]TrackingItem, error) {
	rows, err := d.db.Query("SELECT title, start_from FROM tracking_songs ORDER BY start_from DESC")
	if err != nil {
		return nil, fmt.Errorf("failed to query tracking_songs: %w", err)
	}
	defer rows.Close()

	var items []TrackingItem
	for rows.Next() {
		var item TrackingItem
		if err := rows.Scan(&item.Title, &item.StartFrom); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (d *DB) GetKoreanDictionary() ([]KoreanDictEntry, error) {
	rows, err := d.db.Query("SELECT japanese, korean, stored_at FROM korean_dictionary")
	if err != nil {
		return nil, fmt.Errorf("failed to query korean_dictionary: %w", err)
	}
	defer rows.Close()

	var entries []KoreanDictEntry
	for rows.Next() {
		var entry KoreanDictEntry
		if err := rows.Scan(&entry.Japanese, &entry.Korean, &entry.StoredAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (d *DB) AddTrackingArtist(title string, startFrom time.Time) error {
	_, err := d.db.Exec("INSERT INTO tracking_artists (title, start_from) VALUES ($1, $2)", title, startFrom)
	if err != nil {
		return fmt.Errorf("failed to insert into tracking_artists: %w", err)
	}
	return nil
}

func (d *DB) DeleteTrackingArtist(title string) (int64, error) {
	res, err := d.db.Exec("DELETE FROM tracking_artists WHERE title = $1", title)
	if err != nil {
		return 0, fmt.Errorf("failed to delete from tracking_artists: %w", err)
	}
	return res.RowsAffected()
}

func (d *DB) AddTrackingSong(title string, startFrom time.Time) error {
	_, err := d.db.Exec("INSERT INTO tracking_songs (title, start_from) VALUES ($1, $2)", title, startFrom)
	if err != nil {
		return fmt.Errorf("failed to insert into tracking_songs: %w", err)
	}
	return nil
}

func (d *DB) DeleteTrackingSong(title string) (int64, error) {
	res, err := d.db.Exec("DELETE FROM tracking_songs WHERE title = $1", title)
	if err != nil {
		return 0, fmt.Errorf("failed to delete from tracking_songs: %w", err)
	}
	return res.RowsAffected()
}

func (d *DB) RecordLastUpdated(date time.Time, matched int) error {
	res, err := d.db.Exec("UPDATE last_updated SET matched = $2 WHERE date = $1", date.Format("2006-01-02"), matched)
	if err != nil {
		return fmt.Errorf("failed to update last_updated: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		_, err = d.db.Exec("INSERT INTO last_updated (date, matched) VALUES ($1, $2)", date.Format("2006-01-02"), matched)
		if err != nil {
			return fmt.Errorf("failed to insert into last_updated: %w", err)
		}
	}

	return nil
}

func (d *DB) GetLastUpdatedLogs(limit int) ([]LastUpdated, error) {
	rows, err := d.db.Query("SELECT date, matched FROM last_updated ORDER BY date DESC LIMIT $1", limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query last_updated: %w", err)
	}
	defer rows.Close()

	var logs []LastUpdated
	for rows.Next() {
		var log LastUpdated
		if err := rows.Scan(&log.Date, &log.Matched); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}
