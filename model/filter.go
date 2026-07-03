package model

import "strings"

// matchesFilter reports whether the container matches the case-insensitive
// substring filter against ID and image reference. The filter string is
// only ever compared in memory; it is never passed to a subprocess.
func matchesFilter(c *Container, filter string) bool {
	if filter == "" {
		return true
	}
	f := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(c.ID), f) ||
		strings.Contains(strings.ToLower(c.Image), f)
}
