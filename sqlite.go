package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
)

// One SQLite file, separate indexed tables, WAL transactions and a revision fence.
// The JSON payload retains evidence/history without exposing private user columns.
type sqliteStore struct{ db *sql.DB }

func newSQLiteStore(path string) (*sqliteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	schema := `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS metadata(id INTEGER PRIMARY KEY CHECK(id=1),revision INTEGER NOT NULL);
 INSERT OR IGNORE INTO metadata VALUES(1,0);
 CREATE TABLE IF NOT EXISTS users(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE UNIQUE INDEX IF NOT EXISTS users_steam ON users(json_extract(payload,'$.steamId')) WHERE json_extract(payload,'$.steamId') IS NOT NULL AND json_extract(payload,'$.steamId')<>'';
 CREATE TABLE IF NOT EXISTS records(id TEXT PRIMARY KEY,account TEXT NOT NULL,status TEXT NOT NULL,public INTEGER NOT NULL,community TEXT NOT NULL,updated_at TEXT NOT NULL,payload TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS records_account ON records(account);
 CREATE INDEX IF NOT EXISTS records_listing ON records(public,status,updated_at);
 CREATE TABLE IF NOT EXISTS appeals(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS comments(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS votes(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS communities(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS announcements(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS audit_events(id INTEGER PRIMARY KEY,payload TEXT NOT NULL);`
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	_ = os.Chmod(path, 0600)
	return &sqliteStore{db}, nil
}
func loadRows[T any](ctx context.Context, tx *sql.Tx, table string) ([]T, error) {
	rows, err := tx.QueryContext(ctx, "SELECT payload FROM "+table+" ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var raw string
		var item T
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *sqliteStore) Load(ctx context.Context) (State, int64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return State{}, 0, err
	}
	defer tx.Rollback()
	var rev int64
	var state State
	if err = tx.QueryRowContext(ctx, "SELECT revision FROM metadata WHERE id=1").Scan(&rev); err != nil {
		return state, rev, err
	}
	if state.Users, err = loadRows[User](ctx, tx, "users"); err != nil {
		return state, rev, err
	}
	if state.Records, err = loadRows[Record](ctx, tx, "records"); err != nil {
		return state, rev, err
	}
	if state.Appeals, err = loadRows[Appeal](ctx, tx, "appeals"); err != nil {
		return state, rev, err
	}
	if state.Audit, err = loadRows[Event](ctx, tx, "audit_events"); err != nil {
		return state, rev, err
	}
	if state.Comments, err = loadRows[Comment](ctx, tx, "comments"); err != nil {
		return state, rev, err
	}
	if state.Votes, err = loadRows[Vote](ctx, tx, "votes"); err != nil {
		return state, rev, err
	}
	if state.Communities, err = loadRows[Community](ctx, tx, "communities"); err != nil {
		return state, rev, err
	}
	if state.Announcements, err = loadRows[Announcement](ctx, tx, "announcements"); err != nil {
		return state, rev, err
	}
	return state, rev, tx.Commit()
}
func saveRows[T any](ctx context.Context, tx *sql.Tx, table string, items []T, id func(T) string) error {
	for _, item := range items {
		b, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO "+table+"(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", id(item), string(b)); err != nil {
			return err
		}
	}
	return nil
}
func (s *sqliteStore) Save(ctx context.Context, state State, rev int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE metadata SET revision=revision+1 WHERE id=1 AND revision=?", rev)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return conflict
	}
	if err = saveRows(ctx, tx, "users", state.Users, func(v User) string { return v.Name }); err != nil {
		return err
	}
	for _, v := range state.Records {
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO records(id,account,status,public,community,updated_at,payload) VALUES(?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET account=excluded.account,status=excluded.status,public=excluded.public,community=excluded.community,updated_at=excluded.updated_at,payload=excluded.payload`, v.ID, v.Account, v.Status, v.Public, v.Community, v.UpdatedAt, string(b))
		if e != nil {
			return e
		}
	}
	if err = saveRows(ctx, tx, "appeals", state.Appeals, func(v Appeal) string { return v.ID }); err != nil {
		return err
	}
	if err = saveRows(ctx, tx, "comments", state.Comments, func(v Comment) string { return v.ID }); err != nil {
		return err
	}
	if err = saveRows(ctx, tx, "votes", state.Votes, func(v Vote) string { return v.RecordID + ":" + v.Actor }); err != nil {
		return err
	}
	if err = saveRows(ctx, tx, "communities", state.Communities, func(v Community) string { return v.ID }); err != nil {
		return err
	}
	if err = saveRows(ctx, tx, "announcements", state.Announcements, func(v Announcement) string { return v.ID }); err != nil {
		return err
	}
	for i, v := range state.Audit {
		b, e := json.Marshal(v)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO audit_events(id,payload) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload", i+1, string(b)); e != nil {
			return fmt.Errorf("audit: %w", e)
		}
	}
	return tx.Commit()
}
