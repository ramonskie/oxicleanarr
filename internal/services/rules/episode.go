package rules

// NOTE: EpisodeRule is a future implementation (v1.2).
// The interface and chain slot are defined in engine.go so the engine
// is ready to accept it without structural changes.
//
// EpisodeRule evaluates episode-specific cleanup rules.
// It implements Rule but is placed in the separate episodeRules chain,
// not the standard protection/scheduling chains.
//
// Key differences from standard rules:
//   - Scope() returns ScopeEpisode
//   - Schedule() returns (time.Time{}, 0) — episode rules don't set a whole-item date
//   - The engine reads verdict.EpisodeFileIDs to determine what to delete
//   - Never sees the DiskThresholdRule gate (episode cleanup is count/age-based,
//     not disk-pressure-based)
//
// See FEATURE_EPISODE_CLEANUP_RULES.md for full specification.
