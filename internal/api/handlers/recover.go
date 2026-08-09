package handlers

import (
	"runtime/debug"

	"github.com/rs/zerolog/log"
)

// recoverPanic logs and swallows a panic in a fire-and-forget goroutine so a
// single failure can't crash the whole process. Call with `defer recoverPanic("ctx")`.
func recoverPanic(context string) {
	if recovered := recover(); recovered != nil {
		log.Error().
			Str("context", context).
			Interface("panic", recovered).
			Bytes("stack", debug.Stack()).
			Msg("Recovered from panic in background goroutine")
	}
}
