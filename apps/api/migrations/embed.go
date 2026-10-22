package migrations

import _ "embed"

//go:embed 001_initial.sql
var SQL string

//go:embed 002_email_verification.sql
var emailVerification string

// Files are applied in order at startup, each statement separately. Every
// statement must be idempotent: the same list runs on every boot, and MySQL
// has no ADD COLUMN IF NOT EXISTS, so evolving fields belong in the JSON
// payload rather than in a new column.
func Files() []string { return []string{SQL, emailVerification} }

// Version is the highest migration recorded in schema_migrations.
const Version = 2
