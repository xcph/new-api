// Kimi Search 示例：与 py/scripts/kimi-search.py 一致，使用 Chat Completions 的 builtin $web_search。
//
// 环境变量：
//   MOONSHOT_API_KEY  必填
//   MOONSHOT_BASE_URL 可选，默认 https://api.moonshot.cn/v1（与 Python 示例一致；.ai 与 .cn Key 可能不互通）
//
// 用法：
//   go run . search "你的查询"
//   go run . chat "结合联网搜索回答的问题"
package main

import (
	"fmt"
	"os"

	"github.com/xcph/examples/kimi-search/moonshot"
)

func main() {
	apiKey := os.Getenv("MOONSHOT_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "请设置 MOONSHOT_API_KEY")
		os.Exit(1)
	}
	base := os.Getenv("MOONSHOT_BASE_URL")
	client := moonshot.New(apiKey, base)

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "用法: kimi-search search <query> | kimi-search chat <question>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "用法: kimi-search search <query>")
			os.Exit(2)
		}
		q := os.Args[2]
		out, err := client.WebSearch(q)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(out)
	case "chat":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "用法: kimi-search chat <question>")
			os.Exit(2)
		}
		q := os.Args[2]
		sys := "你是 Kimi，由 Moonshot AI 提供。请根据工具返回的结果回答用户。"
		answer, err := client.ChatWithWebSearch(sys, q)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(answer)
	default:
		fmt.Fprintln(os.Stderr, "未知子命令:", os.Args[1])
		os.Exit(2)
	}
}
