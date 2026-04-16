package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

const hintGlobalAccess404 = `
提示：HTTP 404 且正文含 "Invalid URL" 时，表示请求落到了转发层的未匹配路由（NoRoute），当前进程里**没有注册** /api/global-access/*。
常见原因：运行的是官方 Docker 镜像（calciumion/new-api 等），而 global-access 相关 API 为本仓库 fork 新增，需**使用本仓库源码构建镜像/二进制并部署**后，CLI 才能调用成功。
`

func exitGlobalAccessHTTP(code int, body []byte) {
	s := fmt.Sprintf("HTTP %d: %s", code, truncate(string(body), 2000))
	if code == http.StatusNotFound && strings.Contains(string(body), "Invalid URL") {
		s += hintGlobalAccess404
	}
	fmt.Fprintln(os.Stderr, strings.TrimSpace(s))
	os.Exit(1)
}
