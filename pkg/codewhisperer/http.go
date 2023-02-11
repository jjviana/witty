package codewhisperer

import "net/http"

// BearerHTTPRoundTRipper is a http.RoundTripper that adds a Bearer token to the request.
type BearerHTTPRoundTRipper struct {
	http.RoundTripper
	Token string
}

func (r *BearerHTTPRoundTRipper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer "+r.Token)
	return r.RoundTripper.RoundTrip(req)
}
