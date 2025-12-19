//go:build integration

package twitterscraper

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestGetTweet_UsesLoggedInBranch_RealTweet(t *testing.T) {
	if os.Getenv("SKIP_AUTH_TEST") != "" {
		t.Skip("skipping auth/integration test due to SKIP_AUTH_TEST")
	}

	authToken := os.Getenv("AUTH_TOKEN")
	csrfToken := os.Getenv("CSRF_TOKEN")
	cookiesEnv := os.Getenv("COOKIES")

	if (authToken == "" || csrfToken == "") && cookiesEnv == "" {
		t.Skip("requires AUTH_TOKEN+CSRF_TOKEN or COOKIES env var")
	}

	s := New()

	if cookiesEnv != "" {
		var parsedCookies []*http.Cookie
		if err := json.NewDecoder(strings.NewReader(cookiesEnv)).Decode(&parsedCookies); err != nil {
			t.Fatalf("failed to decode COOKIES json: %v", err)
		}
		s.SetCookies(parsedCookies)
	} else {
		s.SetAuthToken(AuthToken{Token: authToken, CSRFToken: csrfToken})
	}

	if !s.IsLoggedIn() {
		t.Skip("provided auth did not validate via IsLoggedIn()")
	}

	// Force the GetTweet routing decision we care about.
	s.isOpenAccount = false
	s.isLogged = true

	base := http.DefaultTransport
	if s.client.Transport != nil {
		base = s.client.Transport
	}
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.String(), "/i/api/graphql/") || !strings.Contains(req.URL.String(), "/TweetDetail") {
			t.Fatalf("expected TweetDetail logged-in endpoint, got %q", req.URL.String())
		}
		return base.RoundTrip(req)
	})

	const tweetID = "2001787100006690996"
	tw, err := s.GetTweet(tweetID)
	if err != nil {
		t.Fatalf("GetTweet(%s) error = %v", tweetID, err)
	}
	if tw == nil || tw.ID != tweetID {
		t.Fatalf("expected tweet id %q, got %#v", tweetID, tw)
	}
}
