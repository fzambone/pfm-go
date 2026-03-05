// Package db exposes the embedded SQL migration files for use at application startup.
// The embed directive must live here because //go:embed paths cannot use ".." — this
// is the only package location that can reach db/migrations without a parent traversal.
package db

import "embed"

// Migrations holds all SQL migration files from db/migrations/, embedded in the binary
// at build time. Adding a new .sql file to db/migrations/ is sufficient to include it
// on the next build — no registration step required.
//
// all: includes hidden files (e.g. .gitkeep) so the embed succeeds even before any .sql
// files have been added. Without all:, Go's embed skips dot-files and the directive would
// fail on a fresh checkout with no migrations yet.
//go:embed all:migrations
var Migrations embed.FS
