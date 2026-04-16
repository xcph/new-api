package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const ua = "newapi-admin-cli/1.0"

type sessionData struct {
	BaseURL string         `json:"base_url"`
	Cookies []*http.Cookie `json:"cookies"`
	// UserID 与 new-api UserAuth/AdminAuth 要求的 New-Api-User 头一致（与 session cookie 同时生效）
	UserID int `json:"user_id"`
}

func defaultSessionPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "newapi-admin-session.json")
	}
	dir := filepath.Join(home, ".config", "newapi-admin")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "session.json")
}

func loadHTTPClient(baseURL, sessionPath string) (*http.Client, *url.URL, int, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, nil, 0, err
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, 0, err
	}
	b, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, nil, 0, err
	}
	var sd sessionData
	if err := json.Unmarshal(b, &sd); err != nil {
		return nil, nil, 0, err
	}
	if strings.TrimRight(sd.BaseURL, "/") != strings.TrimRight(baseURL, "/") {
		return nil, nil, 0, errMismatchSession
	}
	if sd.UserID <= 0 {
		return nil, nil, 0, errStr("session file missing user_id; please run login again")
	}
	jar.SetCookies(u, sd.Cookies)
	return &http.Client{Jar: jar, Timeout: 120 * time.Second}, u, sd.UserID, nil
}

var errMismatchSession = errStr("session file base_url does not match -base; run login again")

type errStr string

func (e errStr) Error() string { return string(e) }

func saveSession(baseURL, sessionPath string, jar http.CookieJar, u *url.URL, userID int) error {
	sd := sessionData{BaseURL: baseURL, Cookies: jar.Cookies(u), UserID: userID}
	b, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(sessionPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(sessionPath, b, 0600)
}

func httpDo(client *http.Client, userID int, method, urlStr string, body []byte, contentJSON bool) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, urlStr, r)
	if err != nil {
		return nil, 0, err
	}
	if contentJSON && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID > 0 {
		req.Header.Set("New-Api-User", strconv.Itoa(userID))
	}
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

func prettyJSON(b []byte) []byte {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return b
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return b
	}
	return append(out, '\n')
}
