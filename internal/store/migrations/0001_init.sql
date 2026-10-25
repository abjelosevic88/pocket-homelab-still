CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);


-- One practice: a breathing exercise, a sit, a body scan, a mindful walk.
-- Timed by the app or added by hand afterwards. Everything is per person.
CREATE TABLE practices (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id          INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind             TEXT NOT NULL DEFAULT 'meditation',  -- breath | meditation | body_scan | loving_kindness | walking | other
  technique        TEXT NOT NULL DEFAULT '',            -- a breathing pattern or meditation style key, "preset:<id>" for your own
  title            TEXT NOT NULL DEFAULT '',
  started_at       TEXT NOT NULL,                       -- RFC3339 with offset
  ended_at         TEXT NOT NULL DEFAULT '',
  date             TEXT NOT NULL,                       -- local date of started_at
  duration_s       INTEGER NOT NULL DEFAULT 0,          -- time actually practised
  planned_s        INTEGER NOT NULL DEFAULT 0,          -- 0 = open-ended
  completed        INTEGER NOT NULL DEFAULT 1,          -- reached the planned time (or open-ended)
  cycles           INTEGER NOT NULL DEFAULT 0,          -- breathing cycles
  calm_before      INTEGER,                             -- 1 tense … 5 calm
  calm_after       INTEGER,
  hr_before        REAL,
  hr_after         REAL,
  note             TEXT NOT NULL DEFAULT '',
  tags             TEXT NOT NULL DEFAULT '',
  program          TEXT NOT NULL DEFAULT '',            -- a lesson practised for: program key …
  lesson           TEXT NOT NULL DEFAULT '',            -- … and lesson key
  source           TEXT NOT NULL DEFAULT 'app',         -- app | manual | import | <pocket app slug>
  external_id      TEXT NOT NULL DEFAULT '',
  health_synced_at TEXT NOT NULL DEFAULT '',            -- mindful minutes written to Pocket Health
  health_error     TEXT NOT NULL DEFAULT '',
  habit_marked_at  TEXT NOT NULL DEFAULT '',            -- habit rules looked at this practice
  habit_error      TEXT NOT NULL DEFAULT '',
  created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX practices_user_date ON practices(user_id, date);
CREATE INDEX practices_user_started ON practices(user_id, started_at);
CREATE UNIQUE INDEX practices_external ON practices(user_id, source, external_id) WHERE external_id <> '';

-- Your own breathing patterns and timer setups.
CREATE TABLE presets (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL DEFAULT 'timer',   -- breath | timer
  name       TEXT NOT NULL,
  config     TEXT NOT NULL DEFAULT '{}',      -- JSON, see content.BreathConfig / content.TimerConfig
  favorite   INTEGER NOT NULL DEFAULT 0,
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX presets_user ON presets(user_id, kind);

-- Built-in techniques a person starred.
CREATE TABLE favorites (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  PRIMARY KEY (user_id, key)
);

-- Programs (the Path and the short courses): when a person started one and
-- which lessons they finished.
CREATE TABLE program_starts (
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  program    TEXT NOT NULL,
  started_at TEXT NOT NULL,
  PRIMARY KEY (user_id, program)
);

CREATE TABLE lessons_done (
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  program     TEXT NOT NULL,
  lesson      TEXT NOT NULL,
  practice_id INTEGER REFERENCES practices(id) ON DELETE SET NULL,
  done_at     TEXT NOT NULL,
  PRIMARY KEY (user_id, program, lesson)
);

-- Milestones reached (first sit, a 7-day streak, 1000 minutes…).
CREATE TABLE achievements (
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key         TEXT NOT NULL,
  unlocked_at TEXT NOT NULL,
  notified    INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, key)
);

-- "A sit of 10 minutes ticks my Meditate habit": what here marks which Pocket Habits habit.
CREATE TABLE habit_rules (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL DEFAULT 'any',   -- a practice kind or 'any'
  min_minutes REAL NOT NULL DEFAULT 0,       -- the day's minutes of that kind needed
  habit_id    INTEGER NOT NULL,
  habit_name  TEXT NOT NULL DEFAULT '',
  enabled     INTEGER NOT NULL DEFAULT 1,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX habit_rules_user ON habit_rules(user_id);

-- Days a rule already ticked its habit.
CREATE TABLE habit_marks (
  user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  rule_id   INTEGER NOT NULL REFERENCES habit_rules(id) ON DELETE CASCADE,
  date      TEXT NOT NULL,
  marked_at TEXT NOT NULL,
  PRIMARY KEY (user_id, rule_id, date)
);
