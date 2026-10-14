package store

import (
	"database/sql"
	"time"
)

// Test hooks. They live in a _test file so production code cannot reach
// them.

func (s *Store) SetClock(now func() time.Time)           { s.now = now }
func (s *Store) SetFault(fault func(stage string) error) { s.fault = fault }
func (s *Store) DB() *sql.DB                             { return s.db }

var Migrations = migrations

func Migrate(db *sql.DB, steps []string) error { return migrate(db, steps) }
func DSN(path string) string                   { return dsn(path) }
