package redo

import "github.com/bmatcuk/doublestar/v4"

// matchesAny returns true if the path matches any of the given glob patterns.
func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := doublestar.Match(pattern, path); matched {
			return true
		}
	}
	return false
}
