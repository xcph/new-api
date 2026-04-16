package middleware

import (
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const globalAccessScopeEnv = "GLOBAL_ACCESS_SCOPE" // relay（默认）| full

// GlobalAccessGate 按数据库中的全局白/黑名单与模式（Option GlobalAccessListMode）限制访问。
// 默认仅对转发面 /v1、/v1beta、/pg 生效；设置环境变量 GLOBAL_ACCESS_SCOPE=full 时包含 /api（仍豁免部分公开接口）。
func GlobalAccessGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldRunGlobalAccessCheck(c.Request.URL.Path) {
			c.Next()
			return
		}
		mode := model.GetGlobalAccessMode()
		if mode == model.GlobalAccessModeNone || mode == "" {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		apiKey := extractAPIKeyFromRequest(c.Request)

		wl, bl := model.LoadGlobalAccessListsCached()

		switch mode {
		case model.GlobalAccessModeBlacklist:
			if matchesBlacklist(clientIP, apiKey, bl) {
				abortAccessDenied(c)
				return
			}
		case model.GlobalAccessModeWhitelist:
			if !matchesWhitelist(clientIP, apiKey, wl) {
				abortAccessDenied(c)
				return
			}
		}
		c.Next()
	}
}

func abortAccessDenied(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"error": gin.H{
			"type":    "new_api_error",
			"code":    "global_access_denied",
			"message": "access denied by global access list policy",
		},
	})
}

func shouldRunGlobalAccessCheck(path string) bool {
	scope := strings.TrimSpace(os.Getenv(globalAccessScopeEnv))
	if scope == "" {
		scope = "relay"
	}

	if strings.HasPrefix(path, "/v1") || strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/pg") {
		return true
	}

	if scope != "full" {
		return false
	}

	if strings.HasPrefix(path, "/api") {
		return !isGlobalAccessExemptAPI(path)
	}
	return false
}

func isGlobalAccessExemptAPI(path string) bool {
	exempt := []string{
		"/api/status",
		"/api/uptime/status",
		"/api/setup",
	}
	for _, p := range exempt {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func extractAPIKeyFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if r.Header.Get("Sec-WebSocket-Protocol") != "" {
		p := r.Header.Get("Sec-WebSocket-Protocol")
		for _, part := range strings.Split(p, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "openai-insecure-api-key.") {
				return strings.TrimPrefix(part, "openai-insecure-api-key.")
			}
		}
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if k := strings.TrimSpace(r.Header.Get("x-api-key")); k != "" {
		return k
	}
	if k := strings.TrimSpace(r.Header.Get("x-goog-api-key")); k != "" {
		return k
	}
	if strings.HasPrefix(r.URL.Path, "/v1beta") || strings.Contains(r.URL.Path, "/v1/models/") {
		if k := strings.TrimSpace(r.URL.Query().Get("key")); k != "" {
			return k
		}
	}
	if k := strings.TrimSpace(r.Header.Get("mj-api-secret")); k != "" {
		if strings.HasPrefix(strings.ToLower(k), "bearer ") {
			k = strings.TrimSpace(k[7:])
		}
		return k
	}
	return ""
}

func ipMatchesRule(clientIP, rule string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" || clientIP == "" {
		return false
	}
	if strings.Contains(rule, "/") {
		_, ipNet, err := net.ParseCIDR(rule)
		if err != nil {
			return false
		}
		ip := net.ParseIP(clientIP)
		if ip == nil {
			return false
		}
		return ipNet.Contains(ip)
	}
	return clientIP == rule
}

func normalizeKeyFragment(k string) string {
	k = strings.TrimSpace(k)
	k = strings.TrimPrefix(k, "sk-")
	if idx := strings.Index(k, "-"); idx > 0 {
		return k[:idx]
	}
	return k
}

func apiKeyMatches(requestKey, entryValue string) bool {
	requestKey = strings.TrimSpace(requestKey)
	entryValue = strings.TrimSpace(entryValue)
	if requestKey == "" || entryValue == "" {
		return false
	}
	if requestKey == entryValue {
		return true
	}
	rq := normalizeKeyFragment(requestKey)
	ev := normalizeKeyFragment(entryValue)
	return rq != "" && ev != "" && rq == ev
}

func matchesBlacklist(clientIP, apiKey string, entries []*model.GlobalAccessBlacklist) bool {
	for _, e := range entries {
		if e == nil || !e.Enabled {
			continue
		}
		switch e.Type {
		case model.GlobalAccessEntryTypeIP:
			if ipMatchesRule(clientIP, e.Value) {
				return true
			}
		case model.GlobalAccessEntryTypeAPIKey:
			if apiKeyMatches(apiKey, e.Value) {
				return true
			}
		}
	}
	return false
}

func matchesWhitelist(clientIP, apiKey string, entries []*model.GlobalAccessWhitelist) bool {
	if len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e == nil || !e.Enabled {
			continue
		}
		switch e.Type {
		case model.GlobalAccessEntryTypeIP:
			if ipMatchesRule(clientIP, e.Value) {
				return true
			}
		case model.GlobalAccessEntryTypeAPIKey:
			if apiKeyMatches(apiKey, e.Value) {
				return true
			}
		}
	}
	return false
}
