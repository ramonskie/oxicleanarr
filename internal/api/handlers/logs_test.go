package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ramonskie/oxicleanarr/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsHandler_SSE_RotatesOnTruncation(t *testing.T) {
	// Point the log directory at a temp dir and initialise the logger so
	// GetLogDir() resolves to it.
	tmpDir := t.TempDir()
	t.Setenv("LOG_DIR", tmpDir)
	utils.InitLogger("debug", "json", "backend")

	logPath := filepath.Join(tmpDir, "backend.log")
	// 4 initial lines
	require.NoError(t, os.WriteFile(logPath, []byte("{\"level\":\"info\",\"message\":\"l1\"}\n{\"level\":\"info\",\"message\":\"l2\"}\n{\"level\":\"info\",\"message\":\"l3\"}\n{\"level\":\"info\",\"message\":\"l4\"}\n"), 0644))

	handler := NewLogsHandler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &lockedRecorder{rec: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/api/logs?file=backend&stream=true&lines=2", nil).WithContext(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.GetLogs(rec, req)
	}()

	// Wait for initial lines then append a line.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if rec.Len() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Append a new line
	time.Sleep(300 * time.Millisecond)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = f.WriteString("{\"level\":\"info\",\"message\":\"l5\"}\n")
	require.NoError(t, err)
	f.Close()

	// Now truncate the file (simulates rotation)
	time.Sleep(700 * time.Millisecond)
	require.NoError(t, os.WriteFile(logPath, []byte("{\"level\":\"info\",\"message\":\"rotated-l1\"}\n"), 0644))

	// Give the stream time to notice the rotation and emit the notice event.
	time.Sleep(1 * time.Second)
	cancel()
	wg.Wait()

	body := rec.String()
	assert.Contains(t, body, "l4") // last 2 initial lines (l3, l4)
	assert.Contains(t, body, "l5") // appended line
	assert.Contains(t, body, "rotated-l1")
	assert.Contains(t, body, "Log file truncated")
}

// lockedRecorder guards an httptest.ResponseRecorder with a mutex so the SSE
// producer goroutine and the test's reads don't race on the body buffer.
type lockedRecorder struct {
	mu  sync.Mutex
	rec *httptest.ResponseRecorder
}

func (l *lockedRecorder) Header() http.Header {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rec.Header()
}

func (l *lockedRecorder) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rec.Write(b)
}

func (l *lockedRecorder) WriteHeader(code int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rec.WriteHeader(code)
}

func (l *lockedRecorder) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rec.Flush()
}

func (l *lockedRecorder) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rec.Body.Len()
}

func (l *lockedRecorder) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.rec.Body.String()
}

func TestLogsHandler_SSE_EmitsErrorEventWhenFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOG_DIR", tmpDir)
	utils.InitLogger("debug", "json", "backend")

	// InitLogger creates backend.log via lumberjack; remove it so the handler
	// hits the os.Open error path.
	require.NoError(t, os.Remove(filepath.Join(tmpDir, "backend.log")))

	handler := NewLogsHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/logs?file=backend&stream=true", nil)

	handler.GetLogs(rec, req)

	assert.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	assert.True(t, strings.Contains(rec.Body.String(), "event: error"))
}

func TestLogsHandler_SSE_RotatesViaRename(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOG_DIR", tmpDir)
	utils.InitLogger("debug", "json", "backend")

	logPath := filepath.Join(tmpDir, "backend.log")
	require.NoError(t, os.WriteFile(logPath, []byte("{\"level\":\"info\",\"message\":\"l1\"}\n"), 0644))

	handler := NewLogsHandler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &lockedRecorder{rec: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/api/logs?file=backend&stream=true", nil).WithContext(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.GetLogs(rec, req)
	}()

	require.Eventually(t, func() bool {
		return strings.Contains(rec.String(), "\"l1\"")
	}, 3*time.Second, 10*time.Millisecond)

	// Simulate lumberjack rotation: rename the active file aside and create a
	// brand-new file at the same path. The inode swap must be detected even
	// though the old fd still points at the renamed-away file.
	require.NoError(t, os.Rename(logPath, logPath+".1"))
	require.NoError(t, os.WriteFile(logPath, []byte("{\"level\":\"info\",\"message\":\"r1\"}\n"), 0644))

	require.Eventually(t, func() bool {
		return strings.Contains(rec.String(), "Log file rotated") && strings.Contains(rec.String(), "\"r1\"")
	}, 3*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

func TestLogsHandler_SSE_ToleratesOversizedLine(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOG_DIR", tmpDir)
	utils.InitLogger("debug", "json", "backend")

	logPath := filepath.Join(tmpDir, "backend.log")
	bigLine := strings.Repeat("A", 300*1024) // > 256KiB scanner token limit
	require.NoError(t, os.WriteFile(logPath, []byte("{\"level\":\"info\",\"message\":\"l2\"}\n"+bigLine+"\n{\"level\":\"info\",\"message\":\"l3\"}\n"), 0644))

	handler := NewLogsHandler()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rec := &lockedRecorder{rec: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, "/api/logs?file=backend&stream=true", nil).WithContext(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.GetLogs(rec, req)
	}()

	// The line after the oversized one must still reach the client.
	require.Eventually(t, func() bool {
		return strings.Contains(rec.String(), "\"l3\"")
	}, 3*time.Second, 10*time.Millisecond)

	// The stream must still be alive after the oversized line: later appends flow.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = f.WriteString("{\"level\":\"info\",\"message\":\"l4\"}\n")
	require.NoError(t, err)
	f.Close()

	require.Eventually(t, func() bool {
		return strings.Contains(rec.String(), "\"l4\"")
	}, 3*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()

	assert.NotContains(t, rec.String(), "Error reading log file")
}
