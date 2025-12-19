package twitterscraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const bearerToken string = "AAAAAAAAAAAAAAAAAAAAAPYXBAAAAAAACLXUNDekMxqa8h%2F40K4moUkGsoc%3DTYfbDKbT3jJPCEVnMYqilB28NHfOPqkca3qaAxGfsyKCs0wRbw"

// HTTPError is returned when the Twitter API responds with a non-200 status code.
// It preserves the HTTP status and response body for callers that want to implement
// conditional retries (e.g., fallback bearer tokens on 401/403).
type HTTPError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func (e *HTTPError) Error() string {
	// Keep the historical error string format for compatibility.
	return fmt.Sprintf("response status %s: %s", e.Status, e.Body)
}

// RequestAPI get JSON from frontend API and decodes it
func (s *Scraper) RequestAPI(req *http.Request, target interface{}) error {
	s.wg.Wait()
	if s.delay > 0 {
		defer s.delayRequest()
	}

	if err := s.prepareRequest(req); err != nil {
		return err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return s.handleResponse(resp, target)
}

func (s *Scraper) delayRequest() {
	s.wg.Add(1)
	go func() {
		time.Sleep(time.Second * time.Duration(s.delay))
		s.wg.Done()
	}()
}

func (s *Scraper) prepareRequest(req *http.Request) error {
	req.Header.Set("User-Agent", s.userAgent)

	if !s.isLogged {
		if err := s.setGuestToken(req); err != nil {
			return err
		}
	}

	s.setAuthorizationHeader(req)
	s.setCSRFToken(req)

	return nil
}

func (s *Scraper) setGuestToken(req *http.Request) error {
	if !s.IsGuestToken() || s.guestCreatedAt.Before(time.Now().Add(-time.Hour*3)) {
		if err := s.GetGuestToken(); err != nil {
			return err
		}
	}
	req.Header.Set("X-Guest-Token", s.guestToken)
	return nil
}

func (s *Scraper) setAuthorizationHeader(req *http.Request) {
	if s.oAuthToken != "" && s.oAuthSecret != "" {
		req.Header.Set("Authorization", s.sign(req.Method, req.URL))
	} else {
		req.Header.Set("Authorization", "Bearer "+s.bearerToken)
	}
}

func (s *Scraper) setCSRFToken(req *http.Request) {
	for _, cookie := range s.client.Jar.Cookies(req.URL) {
		if cookie.Name == "ct0" {
			req.Header.Set("X-CSRF-Token", cookie.Value)
			break
		}
	}
}

func (s *Scraper) handleResponse(resp *http.Response, target interface{}) error {
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       content,
		}
	}

	if resp.Header.Get("X-Rate-Limit-Remaining") == "0" {
		s.guestToken = ""
	}

	if target == nil {
		return nil
	}

	return json.Unmarshal(content, target)
}

// GetGuestToken from Twitter API
func (s *Scraper) GetGuestToken() error {
	tryTokens := []string{s.bearerToken, bearerToken1, bearerToken2, bearerToken}
	seen := make(map[string]bool)

	var lastErr error
	for _, tok := range tryTokens {
		if tok == "" || seen[tok] {
			continue
		}
		seen[tok] = true

		req, err := http.NewRequest("POST", "https://api.x.com/1.1/guest/activate.json", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("User-Agent", s.userAgent)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("response status %s: %s", resp.Status, body)
			continue
		}

		var jsn map[string]interface{}
		if err := json.Unmarshal(body, &jsn); err != nil {
			return err
		}
		var ok bool
		if s.guestToken, ok = jsn["guest_token"].(string); !ok {
			return fmt.Errorf("guest_token not found")
		}

		// Core fix: ensure subsequent requests use the same bearer token that
		// successfully activated the guest session.
		s.bearerToken = tok
		s.guestCreatedAt = time.Now()
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unable to get guest token")
	}
	return lastErr
}

func (s *Scraper) ClearGuestToken() error {
	s.guestToken = ""
	s.guestCreatedAt = time.Time{}

	return nil
}
