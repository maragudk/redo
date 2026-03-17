package redo

import (
	"testing"

	"maragu.dev/is"
)

func TestMatchesAny(t *testing.T) {
	t.Run("matches Go files with double star glob", func(t *testing.T) {
		is.True(t, matchesAny("main.go", []string{"**/*.go"}))
		is.True(t, matchesAny("cmd/redo/main.go", []string{"**/*.go"}))
	})

	t.Run("matches exact filenames", func(t *testing.T) {
		is.True(t, matchesAny(".env", []string{".env"}))
		is.True(t, matchesAny("go.mod", []string{"go.mod"}))
	})

	t.Run("does not match unrelated files", func(t *testing.T) {
		is.True(t, !matchesAny("style.css", []string{"**/*.go"}))
		is.True(t, !matchesAny("main.go", []string{"**/*.css"}))
	})

	t.Run("matches if any pattern matches", func(t *testing.T) {
		patterns := []string{"**/*.go", "go.mod", "go.sum", ".env"}
		is.True(t, matchesAny("main.go", patterns))
		is.True(t, matchesAny("go.mod", patterns))
		is.True(t, matchesAny(".env", patterns))
		is.True(t, !matchesAny("style.css", patterns))
	})

	t.Run("matches HTML files in subdirectories", func(t *testing.T) {
		is.True(t, matchesAny("views/index.html", []string{"**/*.html"}))
		is.True(t, matchesAny("deep/nested/page.html", []string{"**/*.html"}))
	})

	t.Run("returns false for empty patterns", func(t *testing.T) {
		is.True(t, !matchesAny("anything.go", nil))
		is.True(t, !matchesAny("anything.go", []string{}))
	})
}
