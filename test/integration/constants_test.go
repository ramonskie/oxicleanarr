package integration

// Shared constants for the integration test suite. These used to live in the
// (removed) symlink lifecycle test; other tests depend on them, so they live
// here now.
const (
	ConfigPath     = "../assets/config/config.yaml"
	ComposeFile    = "../assets/docker-compose.yml"
	Phase1Expected = LeavingSoonMovieSymlinks // Items scheduled for deletion with 7d retention
)
