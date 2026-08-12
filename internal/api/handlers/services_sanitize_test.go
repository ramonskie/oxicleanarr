package handlers

import (
	"net/url"
	"testing"
)

func TestSanitizePingErrorScrubsHosts(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"status code preserved", httpStatusErr(401), "unexpected status code: 401"},
		{"connection refused", errWith("dial tcp 192.168.1.50:7878: connect: connection refused"), "connection refused"},
		{"dns lookup", errWith("dial tcp: lookup internal-jellyfin.lan on 127.0.0.11:53: no such host"), "host could not be resolved"},
		{"timeout", errWith("Get \"http://10.0.0.5:8096/System/Info\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)"), "request timed out"},
		{"url embedded in client wrap", errWith("making request to http://jellyfin.internal:8096: dial tcp 10.0.0.5:8096: connect: connection refused"), "connection refused"},
		{"bare hostname in dial error", errWith("dial tcp mynas.lan:7878: connect: network is unreachable"), "network is unreachable"},
		{"single-label host in write error", errWith("write tcp jellyfin:8096->10.0.0.5:443: broken pipe"), "write tcp endpoint->endpoint: broken pipe"},
		{"zone-qualified ipv6", errWith("write tcp [fe80::1%eth0]:8096->[fe80::2%eth0]:443: broken pipe"), "write tcp endpoint->endpoint: broken pipe"},
		{"x509 cert hostname leak", errWith("x509: certificate is valid for *.media.local, not jellyfin.internal"), "certificate verification failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizePingError(tc.err)
			if got != tc.want {
				t.Errorf("sanitizePingError(%q) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

func httpStatusErr(code int) error {
	return &url.Error{URL: "http://x", Err: errWith("unexpected status code: 401")}
}

type strErr string

func (e strErr) Error() string { return string(e) }
func errWith(s string) error   { return strErr(s) }
