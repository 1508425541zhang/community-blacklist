CREATE TABLE IF NOT EXISTS records (
 id TEXT PRIMARY KEY,
 player TEXT NOT NULL,
 account TEXT NOT NULL,
 community TEXT NOT NULL,
 reason TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','discussion','active','appealing','revoked')),
 date TEXT NOT NULL,
 evidence TEXT NOT NULL DEFAULT '',
 public INTEGER NOT NULL DEFAULT 0 CHECK(public IN (0,1))
);
CREATE INDEX IF NOT EXISTS records_public_date ON records(public,date DESC);
CREATE INDEX IF NOT EXISTS records_account ON records(account);
