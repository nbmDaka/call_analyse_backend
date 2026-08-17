// Package migrations exposes the versioned SQL assets used by the migration command.
package migrations

import "embed"

// FS contains all root migration SQL files.
//
//go:embed *.sql
var FS embed.FS
