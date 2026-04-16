package middleware

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

func maskAPIKeyForAccessLog(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}
	// 访问日志保留前 35 位，便于排查；其余字符统一用固定 8 个 * 脱敏。
	if len(raw) <= 35 {
		return raw
	}
	return raw[:35] + "********"
}

// SetUpAccessLogger 注册增强访问日志（client_ip、remote、xff、脱敏 api_key），与上游 logger.go 分离以便合并时减少冲突。
// 与 SetUpLogger 二选一使用，勿同时注册，否则会重复打印。
func SetUpAccessLogger(server *gin.Engine) {
	server.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var requestID string
		if param.Keys != nil {
			requestID, _ = param.Keys[common.RequestIdKey].(string)
		}
		tag, _ := param.Keys[RouteTagKey].(string)
		if tag == "" {
			tag = "web"
		}
		remote := ""
		xff := ""
		apiKeyMasked := "-"
		if param.Request != nil {
			remote = strings.TrimSpace(param.Request.RemoteAddr)
			xff = strings.TrimSpace(param.Request.Header.Get("X-Forwarded-For"))
			if raw := extractAPIKeyFromRequest(param.Request); raw != "" {
				apiKeyMasked = maskAPIKeyForAccessLog(raw)
			}
		}
		return fmt.Sprintf("[GIN] %s | %s | %s | %3d | %13v | client_ip=%s | remote=%s | xff=%s | api_key=%s | %7s %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			tag,
			requestID,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			remote,
			xff,
			apiKeyMasked,
			param.Method,
			param.Path,
		)
	}))
}
