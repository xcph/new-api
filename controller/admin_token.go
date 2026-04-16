package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func parseQueryBool(c *gin.Context, name string) bool {
	v := strings.TrimSpace(strings.ToLower(c.Query(name)))
	return v == "1" || v == "true" || v == "yes"
}

type adminTokenStatusRequest struct {
	Status int `json:"status"`
}

// AdminListAllTokens GET /api/admin/tokens?p=&size=&user_id=
func AdminListAllTokens(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userIdFilter, _ := strconv.Atoi(c.Query("user_id"))
	tokens, total, err := model.ListAllTokensForAdmin(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), userIdFilter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	fullKey := parseQueryBool(c, "full_key")
	items := make([]gin.H, 0, len(tokens))
	for _, t := range tokens {
		if t == nil {
			continue
		}
		uname, _ := model.GetUsernameById(t.UserId, false)
		keyOut := t.GetMaskedKey()
		if fullKey {
			keyOut = t.GetFullKey()
		}
		items = append(items, gin.H{
			"id":                t.Id,
			"user_id":           t.UserId,
			"username":          uname,
			"name":              t.Name,
			"key":               keyOut,
			"status":            t.Status,
			"group":             t.Group,
			"created_time":      t.CreatedTime,
			"accessed_time":     t.AccessedTime,
			"expired_time":      t.ExpiredTime,
			"remain_quota":      t.RemainQuota,
			"used_quota":        t.UsedQuota,
			"unlimited_quota":   t.UnlimitedQuota,
			"model_limits_enabled": t.ModelLimitsEnabled,
		})
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetTokenByID GET /api/admin/tokens/:id
func AdminGetTokenByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	token, err := model.GetTokenById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	uname, _ := model.GetUsernameById(token.UserId, false)
	if parseQueryBool(c, "full_key") {
		common.ApiSuccess(c, gin.H{
			"token":    token,
			"username": uname,
		})
		return
	}
	common.ApiSuccess(c, gin.H{
		"token":    buildMaskedTokenResponse(token),
		"username": uname,
	})
}

// AdminGetTokenDetail GET /api/admin/tokens/:id/detail?start_timestamp=&end_timestamp=
// 默认时间窗口为当前时间往前 24 小时（若未传 start/end）。
func AdminGetTokenDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	token, err := model.GetTokenById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	now := common.GetTimestamp()
	endTs, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	startTs, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if endTs == 0 {
		endTs = now
	}
	if startTs == 0 {
		startTs = endTs - 86400
	}
	if startTs > endTs {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "start_timestamp must be <= end_timestamp"})
		return
	}

	sumQuota, callCount, err := model.GetTokenConsumeAggInRange(id, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ips, err := model.ListDistinctIPsForToken(id, startTs, endTs, 500)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recentLogs, err := model.ListRecentConsumeLogsForToken(id, startTs, endTs, 30)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	uname, _ := model.GetUsernameById(token.UserId, false)
	var tokenOut any = buildMaskedTokenResponse(token)
	if parseQueryBool(c, "full_key") {
		tokenOut = token
	}
	common.ApiSuccess(c, gin.H{
		"token":        tokenOut,
		"username":     uname,
		"period":       gin.H{"start_timestamp": startTs, "end_timestamp": endTs},
		"call_count":   callCount,
		"quota_sum":    sumQuota,
		"distinct_ips": ips,
		"recent_logs":  recentLogs,
	})
}

// AdminUpdateTokenStatus PUT /api/admin/tokens/:id/status
func AdminUpdateTokenStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	req := adminTokenStatusRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status != common.TokenStatusEnabled && req.Status != common.TokenStatusDisabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "status must be 1(enabled) or 2(disabled)"})
		return
	}
	token, err := model.GetTokenById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Status == common.TokenStatusEnabled {
		if token.Status == common.TokenStatusExpired && token.ExpiredTime <= common.GetTimestamp() && token.ExpiredTime != -1 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "token expired, cannot enable"})
			return
		}
		if token.Status == common.TokenStatusExhausted && token.RemainQuota <= 0 && !token.UnlimitedQuota {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "token exhausted, cannot enable"})
			return
		}
	}
	token.Status = req.Status
	if err := token.SelectUpdate(); err != nil {
		common.ApiError(c, err)
		return
	}
	uname, _ := model.GetUsernameById(token.UserId, false)
	common.ApiSuccess(c, gin.H{
		"token":    buildMaskedTokenResponse(token),
		"username": uname,
	})
}
