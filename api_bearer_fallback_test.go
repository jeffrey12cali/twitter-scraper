package twitterscraper

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRequestAPI_BearerFallbackOn401(t *testing.T) {
	s := New()
	s.isLogged = true
	s.isOpenAccount = false

	// Start with bearerToken2 so the first attempt uses it.
	s.setBearerToken(bearerToken2)

	var calls int
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		auth := req.Header.Get("Authorization")

		// First attempt (token2) fails.
		if strings.Contains(auth, "Bearer "+bearerToken2) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`nope`)),
				Request:    req,
			}, nil
		}

		// Fallback should try a different bearer token and succeed.
		if !(strings.Contains(auth, "Bearer "+bearerToken1) || strings.Contains(auth, "Bearer "+bearerToken)) {
			t.Fatalf("expected fallback bearer token, got Authorization=%q", auth)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    req,
		}, nil
	})

	req, err := s.newRequest("GET", "https://x.com/i/api/graphql/some-op")
	if err != nil {
		t.Fatalf("newRequest() error = %v", err)
	}

	// target=nil is fine; we just care about retry behavior.
	if err := s.RequestAPI(req, nil); err != nil {
		t.Fatalf("RequestAPI() error = %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected >=2 calls (initial + fallback), got %d", calls)
	}
}
