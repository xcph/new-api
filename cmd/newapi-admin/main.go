// newapi-admin：new-api 管理员工具（会话登录、global-access、令牌查询）。
//
// 全局参数：
//
//	--base       new-api 根地址（默认 http://127.0.0.1:3000 或 NEWAPI_BASE_URL）
//	--session    会话文件（默认 ~/.config/newapi-admin/session.json）
//
// 示例：
//
//	newapi-admin login --username root --password '***'
//	newapi-admin global-access mode get
//	newapi-admin token list --size 50
//	newapi-admin token detail 12
//	newapi-admin token detail 12 --detail --hours 48
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

func main() {
	root := pflag.NewFlagSet("newapi-admin", pflag.ContinueOnError)
	root.SetInterspersed(false)
	base := root.String("base", getenv("NEWAPI_BASE_URL", "http://127.0.0.1:3000"), "new-api base URL (no trailing slash)")
	session := root.String("session", defaultSessionPath(), "session cookie file")
	help := root.BoolP("help", "h", false, "help")
	root.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: newapi-admin [global options] <command> [command args]

Global options:
`)
		root.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Commands:
  login --username U --password P     管理员登录并保存会话
  logout                              删除本地会话文件
  global-access mode get|set ...
  global-access whitelist list|add|delete ...
  global-access blacklist list|add|delete ...
  token list [--page N] [--size N] [--user-id ID] [--show-token] [--format table|csv|json]
  token detail <id> [--detail] [--hours H] [--start-timestamp] [--end-timestamp] [--show-token] [--json]
  token enable <id>
  token disable <id>

Time format for --start-timestamp / --end-timestamp (local timezone): "2006-01-02 15:04:05"

Examples:
  newapi-admin login -u root -p 'secret'
  newapi-admin global-access mode set whitelist
  newapi-admin token list --size 100
  newapi-admin token list --show-token
  newapi-admin token detail 5
  newapi-admin token detail 5 --detail --hours 72
  newapi-admin token detail 5 --detail --start-timestamp "2026-01-02 10:00:00" --end-timestamp "2026-01-03 18:00:00"
  newapi-admin token disable 5
  newapi-admin token enable 5

`)
	}
	if err := root.Parse(os.Args[1:]); err != nil {
		if err == pflag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *help {
		root.Usage()
		return
	}
	args := root.Args()
	if len(args) == 0 {
		root.Usage()
		os.Exit(2)
	}
	b := strings.TrimRight(*base, "/")

	switch args[0] {
	case "login":
		runLogin(b, *session, args[1:])
	case "logout":
		runLogout(*session)
	case "global-access":
		runGlobalAccess(b, *session, args[1:])
	case "token":
		runToken(b, *session, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		root.Usage()
		os.Exit(2)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
