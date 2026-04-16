package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

func runGlobalAccess(base, sessionPath string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: global-access mode|whitelist|blacklist ...")
		os.Exit(2)
	}
	client, _, userID, err := loadHTTPClient(base, sessionPath)
	if err != nil {
		fatal(err)
	}
	switch args[0] {
	case "mode":
		runGlobalAccessMode(client, userID, base, args[1:])
	case "whitelist":
		runGlobalAccessList(client, userID, base, "whitelist", args[1:])
	case "blacklist":
		runGlobalAccessList(client, userID, base, "blacklist", args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown global-access subcommand: %s\n", args[0])
		os.Exit(2)
	}
}

func runGlobalAccessMode(client *http.Client, userID int, base string, args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: global-access mode get | global-access mode set <none|whitelist|blacklist>")
		os.Exit(2)
	}
	switch args[0] {
	case "get":
		b, code, err := httpDo(client, userID, http.MethodGet, base+"/api/global-access/mode", nil, false)
		if err != nil {
			fatal(err)
		}
		if code != http.StatusOK {
			exitGlobalAccessHTTP(code, b)
		}
		os.Stdout.Write(prettyJSON(b))
	case "set":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: global-access mode set <none|whitelist|blacklist>")
			os.Exit(2)
		}
		mode := strings.ToLower(strings.TrimSpace(args[1]))
		switch mode {
		case "none", "whitelist", "blacklist":
		default:
			fatalf("invalid mode: %s", args[1])
		}
		body, _ := json.Marshal(map[string]string{"mode": mode})
		b, code, err := httpDo(client, userID, http.MethodPut, base+"/api/global-access/mode", body, true)
		if err != nil {
			fatal(err)
		}
		if code != http.StatusOK {
			exitGlobalAccessHTTP(code, b)
		}
		os.Stdout.Write(prettyJSON(b))
	default:
		fmt.Fprintln(os.Stderr, "usage: global-access mode get | global-access mode set ...")
		os.Exit(2)
	}
}

func runGlobalAccessList(client *http.Client, userID int, base, list string, args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: global-access %s list | add | delete ...\n", list)
		os.Exit(2)
	}
	switch args[0] {
	case "list":
		b, code, err := httpDo(client, userID, http.MethodGet, base+"/api/global-access/"+list, nil, false)
		if err != nil {
			fatal(err)
		}
		if code != http.StatusOK {
			exitGlobalAccessHTTP(code, b)
		}
		os.Stdout.Write(prettyJSON(b))
	case "add":
		fs := pflag.NewFlagSet("add", pflag.ExitOnError)
		typ := fs.String("type", "", "ip or api_key")
		val := fs.String("value", "", "value")
		remark := fs.String("remark", "", "optional remark")
		_ = fs.Parse(args[1:])
		if *typ == "" || *val == "" {
			fmt.Fprintln(os.Stderr, "add: --type and --value required")
			os.Exit(2)
		}
		body, _ := json.Marshal(map[string]string{"type": *typ, "value": *val, "remark": *remark})
		b, code, err := httpDo(client, userID, http.MethodPost, base+"/api/global-access/"+list, body, true)
		if err != nil {
			fatal(err)
		}
		if code != http.StatusOK {
			exitGlobalAccessHTTP(code, b)
		}
		os.Stdout.Write(prettyJSON(b))
	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "delete: id required")
			os.Exit(2)
		}
		id := strings.TrimSpace(args[1])
		b, code, err := httpDo(client, userID, http.MethodDelete, base+"/api/global-access/"+list+"/"+id, nil, false)
		if err != nil {
			fatal(err)
		}
		if code != http.StatusOK {
			exitGlobalAccessHTTP(code, b)
		}
		os.Stdout.Write(prettyJSON(b))
	default:
		fmt.Fprintf(os.Stderr, "unknown %s subcommand: %s\n", list, args[0])
		os.Exit(2)
	}
}
