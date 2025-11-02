package config

import (
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

// StartWatcher starts watching the config file for changes
func StartWatcher(onReload func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		defer watcher.Close()

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Watch for write or create events
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Info().Str("file", event.Name).Msg("Config file changed, reloading...")

					if err := Reload(); err != nil {
						log.Error().Err(err).Msg("Failed to reload configuration")
						continue
					}

					// Call the reload callback if provided
					if onReload != nil {
						onReload()
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error().Err(err).Msg("Config watcher error")
			}
		}
	}()

	// Add the config file to the watcher
	if err := watcher.Add(configPath); err != nil {
		return err
	}

	log.Info().Str("path", configPath).Msg("Config file watcher started")
	return nil
}
