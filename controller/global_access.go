package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func GetGlobalAccessMode(c *gin.Context) {
	mode := model.GetGlobalAccessMode()
	if mode == "" {
		mode = model.GlobalAccessModeNone
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"mode": mode,
		},
	})
}

type globalAccessModeReq struct {
	Mode string `json:"mode"`
}

func UpdateGlobalAccessMode(c *gin.Context) {
	var req globalAccessModeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	m := strings.TrimSpace(strings.ToLower(req.Mode))
	switch m {
	case model.GlobalAccessModeNone, model.GlobalAccessModeWhitelist, model.GlobalAccessModeBlacklist:
	default:
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid mode, expect none|whitelist|blacklist"})
		return
	}
	if err := model.UpdateGlobalAccessMode(m); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetGlobalWhitelist(c *gin.Context) {
	list, err := model.ListGlobalWhitelistsAll()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

type globalAccessEntryReq struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Remark string `json:"remark"`
}

func CreateGlobalWhitelist(c *gin.Context) {
	var req globalAccessEntryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	t := strings.TrimSpace(strings.ToLower(req.Type))
	if t != model.GlobalAccessEntryTypeIP && t != model.GlobalAccessEntryTypeAPIKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "type must be ip or api_key"})
		return
	}
	v := strings.TrimSpace(req.Value)
	if v == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "value required"})
		return
	}
	e := &model.GlobalAccessWhitelist{
		Type:    t,
		Value:   v,
		Enabled: true,
		Remark:  req.Remark,
	}
	if err := model.CreateGlobalWhitelist(e); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": e})
}

func DeleteGlobalWhitelist(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	if err := model.DeleteGlobalWhitelist(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func GetGlobalBlacklist(c *gin.Context) {
	list, err := model.ListGlobalBlacklistsAll()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

func CreateGlobalBlacklist(c *gin.Context) {
	var req globalAccessEntryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	t := strings.TrimSpace(strings.ToLower(req.Type))
	if t != model.GlobalAccessEntryTypeIP && t != model.GlobalAccessEntryTypeAPIKey {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "type must be ip or api_key"})
		return
	}
	v := strings.TrimSpace(req.Value)
	if v == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "value required"})
		return
	}
	e := &model.GlobalAccessBlacklist{
		Type:    t,
		Value:   v,
		Enabled: true,
		Remark:  req.Remark,
	}
	if err := model.CreateGlobalBlacklist(e); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": e})
}

func DeleteGlobalBlacklist(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	if err := model.DeleteGlobalBlacklist(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
