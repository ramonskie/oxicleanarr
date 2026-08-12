package handlers

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/utils"
	"github.com/rs/zerolog/log"
)

// LogsHandler handles log file read and streaming requests
type LogsHandler struct{}

// NewLogsHandler creates a new LogsHandler
func NewLogsHandler() *LogsHandler {
	return &LogsHandler{}
}

// LogLine represents a single parsed log line
type LogLine struct {
	Raw       string `json:"raw"`
	Level     string `json:"level,omitempty"`
	Time      string `json:"time,omitempty"`
	Message   string `json:"message,omitempty"`
	Component string `json:"component,omitempty"`
}

// LogsResponse is returned by the static (non-streaming) endpoint
type LogsResponse struct {
	File  string    `json:"file"`
	Lines []LogLine `json:"lines"`
	Total int       `json:"total"`
}

// GetLogs handles GET /api/logs
//
// Query params:
//   - file:   "backend" (default) | "web"
//   - lines:  last N lines to return, default 200, max 2000
//   - stream: "true" → SSE live tail; omit for static snapshot
func (h *LogsHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	logDir := utils.GetLogDir()
	if logDir == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Log directory not available"})
		return
	}

	// Resolve which log file to serve
	fileParam := r.URL.Query().Get("file")
	if fileParam == "" {
		fileParam = "backend"
	}
	var logFileName string
	switch fileParam {
	case "web":
		logFileName = "web.log"
	default:
		fileParam = "backend"
		logFileName = "backend.log"
	}
	logPath := filepath.Join(logDir, logFileName)

	// Number of tail lines
	nLines := 200
	if s := r.URL.Query().Get("lines"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 2000 {
				n = 2000
			}
			nLines = n
		}
	}

	// SSE streaming mode
	if r.URL.Query().Get("stream") == "true" {
		h.streamLogs(w, r, logPath, fileParam)
		return
	}

	// Static snapshot
	lines, err := tailFile(logPath, nLines)
	if err != nil {
		log.Error().Err(err).Str("path", logPath).Msg("Failed to read log file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("Failed to read log file: %s", err.Error())})
		return
	}

	parsed := make([]LogLine, 0, len(lines))
	for _, raw := range lines {
		parsed = append(parsed, parseLine(raw))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(LogsResponse{
		File:  fileParam,
		Lines: parsed,
		Total: len(parsed),
	})
}

// streamLogs tails logPath and pushes new lines to the client via SSE.
// It first sends the last nLines as initial data, then watches for appended content.
func (h *LogsHandler) streamLogs(w http.ResponseWriter, r *http.Request, logPath, fileLabel string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	flusher.Flush()

	// Open the file
	f, err := os.Open(logPath)
	if err != nil {
		sendSSEEvent(w, flusher, "error", fmt.Sprintf(`{"error":"Cannot open log file: %s"}`, err.Error()))
		return
	}
	defer func() { _ = f.Close() }()

	// Send last N initial lines. Reading from the same handle we will tail
	// guarantees no gap between the snapshot and the resume offset.
	nLines := 200
	if s := r.URL.Query().Get("lines"); s != "" {
		if n, err2 := strconv.Atoi(s); err2 == nil && n > 0 {
			if n > 2000 {
				n = 2000
			}
			nLines = n
		}
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		sendSSEEvent(w, flusher, "error", `{"error":"Cannot read log file"}`)
		return
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	initialBuf := make([]string, nLines)
	initialIdx := 0
	initialCount := 0
	for scanner.Scan() {
		raw := scanner.Text()
		if raw == "" {
			continue
		}
		initialBuf[initialIdx%nLines] = raw
		initialIdx++
		initialCount++
	}
	if scanErr := scanner.Err(); scanErr != nil {
		if errors.Is(scanErr, bufio.ErrTooLong) {
			// A single line exceeds the scanner token limit. Skip past it instead
			// of terminating the stream; the snapshot shows what was read before it.
			log.Warn().Err(scanErr).Str("file", logPath).Msg("Oversized log line encountered during initial scan")
			if _, serr := skipToNextLine(f); serr != nil {
				sendSSEEvent(w, flusher, "error", `{"error":"Error reading log file"}`)
				return
			}
		} else {
			sendSSEEvent(w, flusher, "error", `{"error":"Error reading log file"}`)
			return
		}
	}

	// Emit only the last nLines, in chronological order.
	if initialCount <= nLines {
		for i := 0; i < initialCount; i++ {
			ll := parseLine(initialBuf[i])
			data, _ := json.Marshal(ll)
			sendSSEEvent(w, flusher, "log", string(data))
		}
	} else {
		start := initialIdx % nLines
		for i := 0; i < nLines; i++ {
			ll := parseLine(initialBuf[(start+i)%nLines])
			data, _ := json.Marshal(ll)
			sendSSEEvent(w, flusher, "log", string(data))
		}
	}

	// Resume exactly where the snapshot scan stopped, so lines appended while
	// we were reading (or after) are never lost and never duplicated.
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		sendSSEEvent(w, flusher, "error", `{"error":"Error reading log file"}`)
		return
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if the file backing logPath changed identity. Rotation
			// (rename + recreate, e.g. lumberjack) swaps the inode under our fd
			// while the size can stay identical, so size comparison alone misses
			// it. The path may briefly be unavailable mid-rotation, in which
			// case we keepalive and retry.
			pathInfo, perr := os.Stat(logPath)
			info2, err := f.Stat()
			if err != nil {
				sendSSEEvent(w, flusher, "error", `{"error":"Log file became unreadable"}`)
				return
			}

			switch {
			case perr == nil && !os.SameFile(pathInfo, info2):
				// logPath now points at a new inode while our fd still tracks
				// the rotated-away file. Reopen the path and restart from the
				// top of the new file.
				_ = f.Close()
				f, err = os.Open(logPath)
				if err != nil {
					sendSSEEvent(w, flusher, "error", `{"error":"Cannot reopen rotated log file"}`)
					return
				}
				offset = 0
				sendSSEEvent(w, flusher, "notice", `{"message":"Log file rotated, restarting stream"}`)
				continue
			case info2.Size() < offset:
				// File was truncated in place. Reset to the start and emit a
				// marker so the client knows the stream restarted; without this,
				// offset > size would spin in a keepalive loop forever.
				offset = 0
				sendSSEEvent(w, flusher, "notice", `{"message":"Log file truncated, restarting stream"}`)
				fallthrough
			case info2.Size() > offset:
				// Read new bytes
				if _, err := f.Seek(offset, io.SeekStart); err != nil {
					sendSSEEvent(w, flusher, "error", `{"error":"Cannot resume log stream"}`)
					return
				}
				scanner := bufio.NewScanner(f)
				scanner.Buffer(make([]byte, 256*1024), 256*1024)
				for scanner.Scan() {
					raw := scanner.Text()
					if raw == "" {
						continue
					}
					ll := parseLine(raw)
					data, _ := json.Marshal(ll)
					sendSSEEvent(w, flusher, "log", string(data))
				}
				if scanErr := scanner.Err(); scanErr != nil {
					if errors.Is(scanErr, bufio.ErrTooLong) {
						// Skip past the oversized line and resume after it.
						log.Warn().Err(scanErr).Str("file", logPath).Msg("Oversized log line encountered during tail scan")
						if _, serr := skipToNextLine(f); serr != nil {
							sendSSEEvent(w, flusher, "error", `{"error":"Error reading log file"}`)
							return
						}
					} else {
						sendSSEEvent(w, flusher, "error", `{"error":"Error reading log file"}`)
						return
					}
				}
				// Use the exact position where the scanner stopped.
				offset, _ = f.Seek(0, io.SeekCurrent)
			default:
				// No change: send a keepalive comment so proxies don't close the connection
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
			}
		}
	}
}

// sendSSEEvent writes a single SSE event
func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

// skipToNextLine advances f past the end of the current line so a scanner that
// bailed with bufio.ErrTooLong can resume from the following line. It returns
// the new absolute offset of f. The scanner may have over-read into the line's
// tail, so f.Read chunks must not run past the first newline; the position is
// rewound to exactly one byte after it.
func skipToNextLine(f *os.File) (int64, error) {
	start, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	buf := make([]byte, 256*1024)
	pos := start
	for {
		n, err := f.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] == '\n' {
				offset := pos + int64(i) + 1
				_, serr := f.Seek(offset, io.SeekStart)
				return offset, serr
			}
		}
		pos += int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return pos, nil
			}
			return 0, err
		}
	}
}

// tailFile reads the last n lines from the file at path
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	// Read all lines into a circular buffer of size n
	buf := make([]string, n)
	idx := 0
	count := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		buf[idx%n] = line
		idx++
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if count == 0 {
		return []string{}, nil
	}

	// Re-order circular buffer into chronological order
	result := make([]string, 0, min(count, n))
	if count <= n {
		for i := 0; i < count; i++ {
			result = append(result, buf[i])
		}
	} else {
		start := idx % n
		for i := 0; i < n; i++ {
			result = append(result, buf[(start+i)%n])
		}
	}
	return result, nil
}

// parseLine attempts to parse a JSON log line into a LogLine struct.
// Falls back to raw line if the line is not valid JSON.
func parseLine(raw string) LogLine {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return LogLine{Raw: raw}
	}

	ll := LogLine{Raw: raw}
	if v, ok := m["level"].(string); ok {
		ll.Level = v
	}
	if v, ok := m["time"].(string); ok {
		ll.Time = v
	}
	if v, ok := m["message"].(string); ok {
		ll.Message = v
	}
	if v, ok := m["component"].(string); ok {
		ll.Component = v
	}
	return ll
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
