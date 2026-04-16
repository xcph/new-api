package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"

	"github.com/spf13/pflag"
)

func runLogin(base, sessionPath string, args []string) {
	fs := pflag.NewFlagSet("login", pflag.ExitOnError)
	username := fs.StringP("username", "u", "", "admin username")
	password := fs.StringP("password", "p", "", "password")
	_ = fs.Parse(args)
	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "login: --username and --password are required")
		os.Exit(2)
	}
	u, err := url.Parse(base)
	if err != nil {
		fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		fatal(err)
	}
	client := &http.Client{Jar: jar}
	payload := map[string]string{"username": *username, "password": *password}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, base+"/api/user/login", bytes.NewReader(raw))
	if err != nil {
		fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)
	resp, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatalf("login HTTP %s: %s", resp.Status, truncate(string(body), 800))
	}
	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Id         int    `json:"id"`
			Require2FA bool   `json:"require_2fa"`
			Username   string `json:"username"`
			Role       int    `json:"role"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		fatalf("decode: %v\n%s", err, string(body))
	}
	if !out.Success {
		fatalf("login failed: %s", out.Message)
	}
	if out.Data.Require2FA {
		fatalf("account requires 2FA; complete login in browser first")
	}
	if out.Data.Id == 0 {
		fatalf("login response missing user id")
	}
	if out.Data.Role < 10 {
		fatalf("user role=%d is not admin (need >= 10)", out.Data.Role)
	}
	if err := saveSession(base, sessionPath, jar, u, out.Data.Id); err != nil {
		fatal(err)
	}
	fmt.Printf("logged in as %s (role=%d), session: %s\n", out.Data.Username, out.Data.Role, sessionPath)
}

func runLogout(sessionPath string) {
	_ = os.Remove(sessionPath)
	fmt.Println("removed:", sessionPath)
}
