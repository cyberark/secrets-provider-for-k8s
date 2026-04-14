package utils

import (
	"fmt"
	"strconv"
	"strings"
)

// TrimLeadingPathSegments removes leading slash-separated path segments from
// a path string. If trimPathSegments is less than or equal to zero, or greater
// than or equal to the number of path segments, the path is returned unchanged.
func TrimLeadingPathSegments(path string, trimPathSegments int) string {
	if trimPathSegments <= 0 {
		return path
	}

	segments := strings.Split(path, "/")
	if trimPathSegments >= len(segments) {
		return path
	}

	return strings.Join(segments[trimPathSegments:], "/")
}

// ParseTrimPathSegments parses and validates a trim-path-segments value.
// It returns an error when the value cannot be parsed as an integer or is negative.
func ParseTrimPathSegments(rawTrimPathSegments interface{}) (int, error) {
	trimPathSegments, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(rawTrimPathSegments)))
	if err != nil {
		return 0, err
	}

	if trimPathSegments < 0 {
		return 0, fmt.Errorf("expected a non-negative integer")
	}

	return trimPathSegments, nil
}
