package services

import (
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// goRecover runs fn in a goroutine, recovering any panic so it is logged
// instead of crashing the whole process. All fire-and-forget goroutines in the
// service layer must use this.
func goRecover(fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error().
					Interface("panic", recovered).
					Bytes("stack", debug.Stack()).
					Msg("Recovered from panic in background goroutine")
			}
		}()
		fn()
	}()
}

// runSyncSafe runs a single sync invocation, recovering any panic so a failed
// iteration is logged and the scheduler loop continues on its next tick
// instead of crashing the process or silently ending scheduled syncing.
func runSyncSafe(name string, syncFn func() error) {
	if syncFn == nil {
		log.Error().Str("sync", name).Msg("nil sync function passed to runSyncSafe")
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error().
				Str("sync", name).
				Interface("panic", recovered).
				Bytes("stack", debug.Stack()).
				Msg("Recovered from panic during sync iteration")
		}
	}()
	if err := syncFn(); err != nil {
		log.Error().Err(err).Str("sync", name).Msg("Scheduled sync failed")
	}
}
