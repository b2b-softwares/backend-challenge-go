package migrations

import "embed"

// FS contains all SQL migration files embedded in the application.
//
// The migration files are intentionally kept in the migrations directory so
// they remain easy to inspect and version together with the application.
//
//go:embed *.up.sql
var FS embed.FS
