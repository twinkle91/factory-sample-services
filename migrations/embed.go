// Package migrations holds the plain SQL migrations applied on startup.
//
// Files are named NNN_description.sql and run in version order. Never edit a
// migration that has been applied somewhere; add a new file instead.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
