// Package routeallow contains boundary-aware route-prefix matching helpers.
package routeallow

import "strings"

// MatchesPrefix reports whether path is exactly prefix or lies below it at a
// path-segment boundary. Empty prefixes never match.
func MatchesPrefix(path, prefix string) bool {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return false
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = strings.TrimRight(prefix, "/")
	if prefix == "" {
		return path == "/"
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}
