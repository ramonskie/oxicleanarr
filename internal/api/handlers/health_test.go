package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Handle(t *testing.T) {
	t.Run("returns health status successfully", func(t *testing.T) {
		handler := NewHealthHandler()

		// Wait a moment to ensure uptime is measurable
		time.Sleep(10 * time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()

		handler.Handle(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var response HealthResponse
		err := json.NewDecoder(w.Body).Decode(&response)
		require.NoError(t, err)

		assert.Equal(t, "ok", response.Status)
		assert.NotEmpty(t, response.Uptime)
		assert.Equal(t, "1.0.0-dev", response.Version)
	})

	t.Run("uptime increases over time", func(t *testing.T) {
		handler := NewHealthHandler()

		// First request
		req1 := httptest.NewRequest(http.MethodGet, "/health", nil)
		w1 := httptest.NewRecorder()
		handler.Handle(w1, req1)

		var response1 HealthResponse
		json.NewDecoder(w1.Body).Decode(&response1)
		uptime1 := response1.Uptime

		// Wait a bit
		time.Sleep(10 * time.Millisecond)

		// Second request
		req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
		w2 := httptest.NewRecorder()
		handler.Handle(w2, req2)

		var response2 HealthResponse
		json.NewDecoder(w2.Body).Decode(&response2)
		uptime2 := response2.Uptime

		// Uptime should have increased
		assert.NotEqual(t, uptime1, uptime2)
	})

	t.Run("returns consistent version", func(t *testing.T) {
		handler := NewHealthHandler()

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			w := httptest.NewRecorder()
			handler.Handle(w, req)

			var response HealthResponse
			json.NewDecoder(w.Body).Decode(&response)
			assert.Equal(t, "1.0.0-dev", response.Version)
		}
	})
}
