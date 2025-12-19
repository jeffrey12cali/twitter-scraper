package twitterscraper

import (
	"encoding/json"
	"testing"
)

func TestThreadedConversationParse_SupportsInstructionEntry(t *testing.T) {
	const focalID = "2001787100006690996"

	// Minimal TweetDetail-shaped JSON where the tweet is under `instructions[].entry`
	// (not `instructions[].entries`).
	raw := []byte(`{
  "data": {
    "threaded_conversation_with_injections_v2": {
      "instructions": [
        {
          "type": "TimelineAddEntry",
          "entry": {
            "content": {
              "itemContent": {
                "itemType": "TimelineTweet",
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
                      "id_str": "` + focalID + `",
                      "conversation_id_str": "` + focalID + `",
                      "user_id_str": "u1",
                      "created_at": "Mon Jan 02 15:04:05 -0700 2006",
                      "full_text": "hello"
                    }
                  }
                }
              }
            }
          }
        }
      ]
    }
  }
}`)

	var conv threadedConversation
	if err := json.Unmarshal(raw, &conv); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	tweets, _ := conv.parse(focalID)
	if len(tweets) != 1 {
		t.Fatalf("expected 1 tweet parsed, got %d", len(tweets))
	}
	if tweets[0].ID != focalID {
		t.Fatalf("expected tweet id %q, got %q", focalID, tweets[0].ID)
	}
}
