package twitterscraper

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetTweet_UsesLoggedInBranch(t *testing.T) {
	const tweetID = "123"

	s := New()
	s.isLogged = true // force GetTweet to use the logged-in branch

	var called int
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		called++
		if !strings.Contains(req.URL.String(), "/i/api/graphql/") || !strings.Contains(req.URL.String(), "/TweetDetail") {
			t.Fatalf("expected TweetDetail logged-in endpoint, got %q", req.URL.String())
		}

		// Ensure the request encodes the focal tweet ID into the variables parameter.
		if gotVars := req.URL.Query().Get("variables"); !strings.Contains(gotVars, `"focalTweetId":"`+tweetID+`"`) {
			t.Fatalf("expected variables to contain focalTweetId=%q, got %q", tweetID, gotVars)
		}

		// Minimal JSON response that satisfies threadedConversation.parse() and parseLegacyTweet().
		body := `{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "type": "TimelineAddEntries",
          "entries": [
            {
              "content": {
                "itemContent": {
                  "tweet_results": {
                    "result": {
                      "__typename": "Tweet",
                      "core": {
                        "user_results": {
                          "result": {
                            "legacy": {
                              "id_str": "u1",
                              "screen_name": "user",
                              "name": "User"
                            }
                          }
                        }
                      },
                      "legacy": {
                        "id_str": "` + tweetID + `",
                        "conversation_id_str": "` + tweetID + `",
                        "user_id_str": "u1",
                        "created_at": "Mon Jan 02 15:04:05 -0700 2006",
                        "full_text": "hello from logged branch"
                      }
                    }
                  }
                }
              }
            }
          ]
        }
      ]
    }
  }
}`

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})

	tw, err := s.GetTweet(tweetID)
	if err != nil {
		t.Fatalf("GetTweet() error = %v", err)
	}
	if tw == nil || tw.ID != tweetID {
		t.Fatalf("expected tweet id %q, got %#v", tweetID, tw)
	}
	if tw.Username != "user" {
		t.Fatalf("expected username %q, got %q", "user", tw.Username)
	}
	if called != 1 {
		t.Fatalf("expected exactly 1 HTTP request, got %d", called)
	}
}

func TestGetTweet_UsesLoggedInBranch_RetriesOn401FromBearerToken2(t *testing.T) {
	const tweetID = "123"

	s := New()
	s.isLogged = true
	s.isOpenAccount = false
	originalBearer := bearerToken1
	s.setBearerToken(originalBearer)

	var calls int
	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++

		// Ensure we are in the logged-in TweetDetail branch.
		if !strings.Contains(req.URL.String(), "/i/api/graphql/") || !strings.Contains(req.URL.String(), "/TweetDetail") {
			t.Fatalf("expected TweetDetail logged-in endpoint, got %q", req.URL.String())
		}

		// First attempt uses bearerToken2 (due to the swap in GetTweet) and fails with 401.
		auth := req.Header.Get("Authorization")
		if strings.Contains(auth, "Bearer "+bearerToken2) {
			resp := &http.Response{
				StatusCode: http.StatusUnauthorized,
				Status:     "401 Unauthorized",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"Could not authenticate you","code":32}]}`)),
				Request:    req,
			}
			resp.Header.Set("Content-Type", "application/json")
			return resp, nil
		}

		// Retry should fall back to the original bearer token and succeed.
		if !strings.Contains(auth, "Bearer "+originalBearer) {
			t.Fatalf("expected retry to use original bearer %q, got Authorization=%q", originalBearer, auth)
		}

		body := `{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "type": "TimelineAddEntries",
          "entries": [
            {
              "content": {
                "itemContent": {
                  "tweet_results": {
                    "result": {
                      "__typename": "Tweet",
                      "core": {
                        "user_results": {
                          "result": {
                            "legacy": {
                              "id_str": "u1",
                              "screen_name": "user",
                              "name": "User"
                            }
                          }
                        }
                      },
                      "legacy": {
                        "id_str": "` + tweetID + `",
                        "conversation_id_str": "` + tweetID + `",
                        "user_id_str": "u1",
                        "created_at": "Mon Jan 02 15:04:05 -0700 2006",
                        "full_text": "hello from logged branch"
                      }
                    }
                  }
                }
              }
            }
          ]
        }
      ]
    }
  }
}`

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})

	tw, err := s.GetTweet(tweetID)
	if err != nil {
		t.Fatalf("GetTweet() error = %v", err)
	}
	if tw == nil || tw.ID != tweetID {
		t.Fatalf("expected tweet id %q, got %#v", tweetID, tw)
	}
	if calls != 2 {
		t.Fatalf("expected 2 HTTP calls (401 then retry), got %d", calls)
	}
	if s.bearerToken != originalBearer {
		t.Fatalf("expected scraper bearer token restored to %q, got %q", originalBearer, s.bearerToken)
	}
}

func TestGetTweet_LoggedInBranch_ReturnsGraphQLErrors(t *testing.T) {
	const tweetID = "2001787100006690996"

	s := New()
	s.isLogged = true
	s.isOpenAccount = false
	s.setBearerToken(bearerToken1)

	s.client.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Ensure we're targeting TweetDetail.
		if !strings.Contains(req.URL.String(), "/i/api/graphql/") || !strings.Contains(req.URL.String(), "/TweetDetail") {
			t.Fatalf("expected TweetDetail logged-in endpoint, got %q", req.URL.String())
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"Some graphql error","code":123}],"data":{"threaded_conversation_with_injections_v2":null}}`)),
			Request:    req,
		}
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})

	_, err := s.GetTweet(tweetID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "graphql error") {
		t.Fatalf("expected graphql error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Some graphql error") {
		t.Fatalf("expected graphql error message to be surfaced, got: %v", err)
	}
}
