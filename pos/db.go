package pos

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS transactions (
	id                   TEXT PRIMARY KEY,
	state                INTEGER NOT NULL,
	amount_cents         INTEGER NOT NULL,
	currency             TEXT NOT NULL,
	card_holder_name     TEXT NOT NULL DEFAULT '',
	card_last_four       TEXT NOT NULL DEFAULT '',
	card_has_loyalty     INTEGER NOT NULL DEFAULT 0,
	card_loyalty_points  INTEGER NOT NULL DEFAULT 0,
	selected_bank_id     INTEGER NOT NULL DEFAULT 0,
	selected_aid         TEXT NOT NULL DEFAULT '',
	installments         INTEGER NOT NULL DEFAULT 1,
	use_loyalty_points   INTEGER NOT NULL DEFAULT 0,
	loyalty_points_used  INTEGER NOT NULL DEFAULT 0,
	loyalty_amount_cents INTEGER NOT NULL DEFAULT 0,
	card_amount_cents    INTEGER NOT NULL DEFAULT 0,
	confirmation_code    TEXT NOT NULL DEFAULT '',
	receipt_number       TEXT NOT NULL DEFAULT '',
	auth_code            TEXT NOT NULL DEFAULT '',
	error_message        TEXT NOT NULL DEFAULT '',
	created_at           TEXT NOT NULL,
	completed_at         TEXT NOT NULL DEFAULT '',
	last_updated         TEXT NOT NULL,
	cached_response      BLOB,
	z_report_id          INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS metadata (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);`

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
