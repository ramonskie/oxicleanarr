package rules

import (
	"time"

	"github.com/ramonskie/oxicleanarr/internal/models"
)

// equalsCaseInsensitive compares two strings case-insensitively.
func equalsCaseInsensitive(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return toLower(a) == toLower(b)
}

// toLower converts a string to lowercase without importing strings package.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// clampToNow prevents future base times (clock skew / data errors)
// from producing negative retention periods.
func clampToNow(t time.Time) time.Time {
	if t.After(time.Now()) {
		return time.Now()
	}
	return t
}

// scopeMatches returns true if the rule's scope applies to the given media type.
func scopeMatches(scope RuleScope, mediaType models.MediaType) bool {
	switch scope {
	case ScopeAll:
		return true
	case ScopeMovies:
		return mediaType == models.MediaTypeMovie
	case ScopeTVShows, ScopeEpisode:
		return mediaType == models.MediaTypeTVShow
	default:
		return false
	}
}
