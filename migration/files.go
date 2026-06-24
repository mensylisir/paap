package migration

import "embed"

// Files contains PAAP database migration SQL files.
//
//go:embed *.sql
var Files embed.FS
