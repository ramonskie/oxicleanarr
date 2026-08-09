package handlers

import "sync"

// configWriteMu serializes config read-modify-write cycles (e.g. rule CRUD) so
// two concurrent writes can't silently overwrite each other's changes. A second
// writer waits for the first to finish before re-reading the updated config.
var configWriteMu sync.Mutex
