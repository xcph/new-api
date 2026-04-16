package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// 与 flag 说明一致：本地时区的日期时间字符串。
const timeLayoutDateTime = "2006-01-02 15:04:05"

func parseLocalDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	return time.ParseInLocation(timeLayoutDateTime, s, time.Local)
}

func runToken(base, sessionPath string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: token list | token detail <id> ... | token enable <id> | token disable <id>")
		os.Exit(2)
	}
	client, _, userID, err := loadHTTPClient(base, sessionPath)
	if err != nil {
		fatal(err)
	}
	switch args[0] {
	case "list":
		runTokenList(client, userID, base, args[1:])
	case "detail":
		runTokenDetail(client, userID, base, args[1:])
	case "enable":
		runTokenSetStatus(client, userID, base, args[1:], 1)
	case "disable":
		runTokenSetStatus(client, userID, base, args[1:], 2)
	default:
		fmt.Fprintf(os.Stderr, "unknown token subcommand: %s\n", args[0])
		os.Exit(2)
	}
}

func runTokenSetStatus(client *http.Client, userID int, base string, args []string, status int) {
	if len(args) < 1 {
		if status == 1 {
			fmt.Fprintln(os.Stderr, "usage: token enable <id>")
		} else {
			fmt.Fprintln(os.Stderr, "usage: token disable <id>")
		}
		os.Exit(2)
	}
	id := strings.TrimSpace(args[0])
	body, _ := json.Marshal(map[string]int{"status": status})
	u := base + "/api/admin/tokens/" + url.PathEscape(id) + "/status"
	b, code, err := httpDo(client, userID, http.MethodPut, u, body, true)
	if err != nil {
		fatal(err)
	}
	if code != http.StatusOK {
		fatalf("HTTP %d: %s", code, string(b))
	}
	os.Stdout.Write(prettyJSON(b))
}

func runTokenList(client *http.Client, userID int, base string, args []string) {
	fs := pflag.NewFlagSet("list", pflag.ExitOnError)
	page := fs.Int("page", 1, "page number (1-based)")
	size := fs.Int("size", 20, "page size")
	filterUser := fs.Int("user-id", 0, "filter by user id (optional)")
	showToken := fs.Bool("show-token", false, "request full API key from server (admin API adds ?full_key=1)")
	format := fs.String("format", "table", "output format: table | csv | json")
	_ = fs.Parse(args)
	q := url.Values{}
	q.Set("p", strconv.Itoa(*page))
	q.Set("size", strconv.Itoa(*size))
	if *filterUser > 0 {
		q.Set("user_id", strconv.Itoa(*filterUser))
	}
	if *showToken {
		q.Set("full_key", "1")
	}
	u := base + "/api/admin/tokens?" + q.Encode()
	b, code, err := httpDo(client, userID, http.MethodGet, u, nil, false)
	if err != nil {
		fatal(err)
	}
	if code != http.StatusOK {
		fatalf("HTTP %d: %s", code, string(b))
	}
	data, err := parseAPISuccessData(b)
	if err != nil {
		fatal(err)
	}
	var payload tokenListPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		fatal(err)
	}
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "table":
		printTokenListTable(os.Stdout, &payload)
	case "csv":
		if err := printTokenListCSV(os.Stdout, &payload); err != nil {
			fatal(err)
		}
	case "json":
		if err := printTokenListJSON(os.Stdout, &payload); err != nil {
			fatal(err)
		}
	default:
		fatalf("invalid --format: %s (allowed: table,csv,json)", *format)
	}
}

func runTokenDetail(client *http.Client, userID int, base string, args []string) {
	fs := pflag.NewFlagSet("detail", pflag.ExitOnError)
	detail := fs.Bool("detail", false, "fetch extended report: period quota, call count, distinct IPs, recent logs (GET .../tokens/:id/detail)")
	hours := fs.Float64("hours", 24, "with --detail: time window length when start/end not set")
	startStr := fs.String("start-timestamp", "", `with --detail: range start, format "2006-01-02 15:04:05" (local timezone); use with --end-timestamp`)
	endStr := fs.String("end-timestamp", "", `with --detail: range end, same format as --start-timestamp`)
	showToken := fs.Bool("show-token", false, "request full API key from server (?full_key=1)")
	asJSON := fs.Bool("json", false, "print raw JSON response instead of formatted text")
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "usage: token detail <id> [--detail] [--hours H] [--start-timestamp] [--end-timestamp] [--show-token] [--json]")
		os.Exit(2)
	}
	id := rest[0]
	if !*detail {
		u := base + "/api/admin/tokens/" + url.PathEscape(id)
		if *showToken {
			u += "?full_key=1"
		}
		b, code, err := httpDo(client, userID, http.MethodGet, u, nil, false)
		if err != nil {
			fatal(err)
		}
		if code != http.StatusOK {
			fatalf("HTTP %d: %s", code, string(b))
		}
		if *asJSON {
			os.Stdout.Write(prettyJSON(b))
			return
		}
		data, err := parseAPISuccessData(b)
		if err != nil {
			fatal(err)
		}
		var wrap struct {
			Token    adminTokenRow `json:"token"`
			Username string        `json:"username"`
		}
		if err := json.Unmarshal(data, &wrap); err != nil {
			fatal(err)
		}
		printTokenDetailBlock(os.Stdout, wrap.Username, &wrap.Token)
		return
	}

	q := url.Values{}
	startIn := strings.TrimSpace(*startStr)
	endIn := strings.TrimSpace(*endStr)
	if startIn != "" || endIn != "" {
		if startIn == "" || endIn == "" {
			fatalf("set both --start-timestamp and --end-timestamp, or neither (then --hours applies)")
		}
		tStart, err := parseLocalDateTime(startIn)
		if err != nil {
			fatalf("--start-timestamp: %v (expected format %q)", err, timeLayoutDateTime)
		}
		tEnd, err := parseLocalDateTime(endIn)
		if err != nil {
			fatalf("--end-timestamp: %v (expected format %q)", err, timeLayoutDateTime)
		}
		q.Set("start_timestamp", strconv.FormatInt(tStart.Unix(), 10))
		q.Set("end_timestamp", strconv.FormatInt(tEnd.Unix(), 10))
	} else {
		end := time.Now().Unix()
		start := int64(float64(end) - *hours*3600)
		q.Set("start_timestamp", strconv.FormatInt(start, 10))
		q.Set("end_timestamp", strconv.FormatInt(end, 10))
	}
	if *showToken {
		q.Set("full_key", "1")
	}
	u := base + "/api/admin/tokens/" + url.PathEscape(id) + "/detail?" + q.Encode()
	b, code, err := httpDo(client, userID, http.MethodGet, u, nil, false)
	if err != nil {
		fatal(err)
	}
	if code != http.StatusOK {
		fatalf("HTTP %d: %s", code, string(b))
	}
	if *asJSON {
		os.Stdout.Write(prettyJSON(b))
		return
	}
	data, err := parseAPISuccessData(b)
	if err != nil {
		fatal(err)
	}
	var ext tokenDetailExt
	if err := json.Unmarshal(data, &ext); err != nil {
		fatal(err)
	}
	printTokenDetailExtended(os.Stdout, &ext)
}
