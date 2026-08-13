package integration

// Leaving Soon plugin integration constants. The plugin (jellyfin-plugin-leaving-soon)
// pulls leaving-soon data from provider apps and manages the symlink libraries itself.
const (
	// Container path inside the Jellyfin container where symlinks are created.
	// Matches the compose mount ./leaving-soon:/app/leaving-soon.
	LeavingSoonBasePath = "/app/leaving-soon"
	// The Leaving Soon plugin's folder name inside Jellyfin's plugins directory.
	LeavingSoonPluginFolder = "Leaving Soon"
	// Expected movies scheduled with 7d retention (Phase1Expected aliases this).
	LeavingSoonMovieSymlinks = 7
	// OxiCleanarr container hostname as seen from the Jellyfin container.
	LeavingSoonOxiCleanarrURL = "http://oxicleanarr-test:9709"
	// Static Bearer key the plugin uses against OxiCleanarr. Must match
	// admin.api_key in test/assets/config/config.yaml.
	OxiCleanarrTestAPIKey = "test-api-key"
	// The Leaving Soon plugin's GUID (dashed, matches meta.json + Plugin.Id).
	LeavingSoonPluginGUID = "b31d2e5a-8f4e-4c6a-b7a3-2d4e5f6a7b8c"
	// Jellyfin registers plugin ids WITHOUT dashes (verified on a live 10.11
	// instance); used for the /Plugins/{id}/Configuration API path.
	LeavingSoonPluginAPIID = "b31d2e5a8f4e4c6ab7a32d4e5f6a7b8c"
	// Library names the install helper writes into the plugin config.xml.
	LeavingSoonMoviesLibrary = "Test - Leaving Soon Movies"
	LeavingSoonTVLibrary     = "Test - Leaving Soon TV"
)
