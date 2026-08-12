package integration

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testAuthEndpoints exercises the auth hardening behaviors: non-JSON login
// rejection (415) and the HttpOnly session cookie used by the web UI.
func testAuthEndpoints(t *testing.T) {
	t.Run("RejectsNonJSONContentType", func(t *testing.T) {
		resp, data := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/auth/login", "",
			"application/x-www-form-urlencoded", []byte("username=admin&password=adminpassword"))
		assert.Equal(t, http.StatusUnsupportedMediaType, resp.StatusCode, "non-JSON login must be rejected with 415")
		assert.Contains(t, string(data), "application/json")

		// A valid JSON login must still work (regression guard for the 415 path).
		token := loginToken(t)
		assert.NotEmpty(t, token)
	})

	t.Run("SetsHttpOnlyCookie", func(t *testing.T) {
		resp, _ := rawRequest(t, http.MethodPost, OxiCleanarrURL+"/api/auth/login", "",
			"application/json", []byte(`{"username":"admin","password":"adminpassword"}`))
		require.Equal(t, http.StatusOK, resp.StatusCode)
		// The web UI authenticates via the session cookie; it must be HttpOnly
		// so the token stays out of JavaScript.
		setCookie := resp.Header.Get("Set-Cookie")
		assert.Contains(t, setCookie, "HttpOnly", "auth cookie must be HttpOnly")
		assert.Contains(t, setCookie, "SameSite", "auth cookie must set SameSite")
		assert.Contains(t, setCookie, "Path=/", "auth cookie must be sent on all paths")
	})
}
