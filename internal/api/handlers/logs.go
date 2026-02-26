package handlers

import (
	"bufio"
	"encoding/json"
	"fmt"
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

	// Open the file and seek to the position after the last N lines
	f, err := os.Open(logPath)
	if err != nil {
		sendSSEEvent(w, flusher, "error", fmt.Sprintf(`{"error":"Cannot open log file: %s"}`, err.Error()))
		return
	}
	defer f.Close()

	// Send last N initial lines
	nLines := 200
	if s := r.URL.Query().Get("lines"); s != "" {
		if n, err2 := strconv.Atoi(s); err2 == nil && n > 0 {
			if n > 2000 {
				n = 2000
			}
			nLines = n
		}
	}

	initialLines, _ := tailFile(logPath, nLines)
	for _, raw := range initialLines {
		ll := parseLine(raw)
		data, _ := json.Marshal(ll)
		sendSSEEvent(w, flusher, "log", string(data))
	}

	// Seek to end of file so we only tail new writes
	info, err := f.Stat()
	if err != nil {
		return
	}
	offset := info.Size()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if file has grown
			info2, err := f.Stat()
			if err != nil {
				return
			}
			if info2.Size() <= offset {
				// Send a keepalive comment so proxies don't close the connection
				fmt.Fprintf(w, ": keepalive\n\n")
				flusher.Flush()
				continue
			}

			// Read new bytes
			if _, err := f.Seek(offset, 0); err != nil {
				return
			}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				raw := scanner.Text()
				if raw == "" {
					continue
				}
				ll := parseLine(raw)
				data, _ := json.Marshal(ll)
				sendSSEEvent(w, flusher, "log", string(data))
			}
			newInfo, _ := f.Stat()
			if newInfo != nil {
				offset = newInfo.Size()
			}
		}
	}
}

// sendSSEEvent writes a single SSE event
func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event, data string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
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
