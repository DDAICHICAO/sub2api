package repository

import (
	"net/http"
	"strings"
)

// upstreamRequestIdentity keeps gateway-only controls off the wire. Clone the
// headers so local cache/transport controls on the caller's request still work.
// Request bodies and provider-required client identities are not rewritten.
func upstreamRequestIdentity(req *http.Request) *http.Request {
	if req == nil {
		return nil
	}
	out := req.Clone(req.Context())
	for name := range out.Header {
		if strings.HasPrefix(strings.ToLower(name), "x-sub2api-") {
			delete(out.Header, name)
		}
	}
	if strings.Contains(strings.ToLower(out.UserAgent()), "sub2api") {
		out.Header.Set("User-Agent", "API-Client/1.0")
	}
	return out
}
