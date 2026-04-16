package model

import (
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// GlobalAccessListMode 全局访问控制模式，存于 options 表 key=GlobalAccessListMode。
const (
	GlobalAccessModeNone       = "none"
	GlobalAccessModeWhitelist  = "whitelist"
	GlobalAccessModeBlacklist  = "blacklist"
	GlobalAccessEntryTypeIP    = "ip"
	GlobalAccessEntryTypeAPIKey = "api_key"
)

// GlobalAccessWhitelist 全局白名单（IP 或 API Key）。
type GlobalAccessWhitelist struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type      string `json:"type" gorm:"size:16;index;not null"` // ip | api_key
	Value     string `json:"value" gorm:"type:text;not null"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	Remark    string `json:"remark" gorm:"size:512"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (GlobalAccessWhitelist) TableName() string {
	return "global_access_whitelists"
}

// GlobalAccessBlacklist 全局黑名单。
type GlobalAccessBlacklist struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Type      string `json:"type" gorm:"size:16;index;not null"`
	Value     string `json:"value" gorm:"type:text;not null"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	Remark    string `json:"remark" gorm:"size:512"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (GlobalAccessBlacklist) TableName() string {
	return "global_access_blacklists"
}

func GetGlobalAccessMode() string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	if common.OptionMap == nil {
		return GlobalAccessModeNone
	}
	m := common.OptionMap["GlobalAccessListMode"]
	if m == "" {
		return GlobalAccessModeNone
	}
	return m
}

var (
	globalAccessListsMu     sync.RWMutex
	globalAccessWhitelist   []*GlobalAccessWhitelist
	globalAccessBlacklist   []*GlobalAccessBlacklist
	globalAccessListsLoaded time.Time
)

const globalAccessListCacheTTL = 30 * time.Second

// LoadGlobalAccessListsCached 供中间件使用，带短 TTL 缓存。
func LoadGlobalAccessListsCached() (w []*GlobalAccessWhitelist, b []*GlobalAccessBlacklist) {
	globalAccessListsMu.RLock()
	if time.Since(globalAccessListsLoaded) < globalAccessListCacheTTL && globalAccessListsLoaded.Unix() > 0 {
		w, b = globalAccessWhitelist, globalAccessBlacklist
		globalAccessListsMu.RUnlock()
		return
	}
	globalAccessListsMu.RUnlock()

	globalAccessListsMu.Lock()
	defer globalAccessListsMu.Unlock()
	if time.Since(globalAccessListsLoaded) < globalAccessListCacheTTL && globalAccessListsLoaded.Unix() > 0 {
		return globalAccessWhitelist, globalAccessBlacklist
	}
	var wl []*GlobalAccessWhitelist
	var bl []*GlobalAccessBlacklist
	_ = DB.Where("enabled = ?", true).Order("id").Find(&wl).Error
	_ = DB.Where("enabled = ?", true).Order("id").Find(&bl).Error
	globalAccessWhitelist = wl
	globalAccessBlacklist = bl
	globalAccessListsLoaded = time.Now()
	return globalAccessWhitelist, globalAccessBlacklist
}

// InvalidateGlobalAccessListCache 在管理端增删改后调用。
func InvalidateGlobalAccessListCache() {
	globalAccessListsMu.Lock()
	defer globalAccessListsMu.Unlock()
	globalAccessListsLoaded = time.Time{}
	globalAccessWhitelist = nil
	globalAccessBlacklist = nil
}

func ListGlobalWhitelistsAll() ([]*GlobalAccessWhitelist, error) {
	var list []*GlobalAccessWhitelist
	err := DB.Order("id").Find(&list).Error
	return list, err
}

func ListGlobalBlacklistsAll() ([]*GlobalAccessBlacklist, error) {
	var list []*GlobalAccessBlacklist
	err := DB.Order("id").Find(&list).Error
	return list, err
}

func CreateGlobalWhitelist(e *GlobalAccessWhitelist) error {
	now := time.Now().Unix()
	e.CreatedAt = now
	e.UpdatedAt = now
	err := DB.Create(e).Error
	if err == nil {
		InvalidateGlobalAccessListCache()
	}
	return err
}

func CreateGlobalBlacklist(e *GlobalAccessBlacklist) error {
	now := time.Now().Unix()
	e.CreatedAt = now
	e.UpdatedAt = now
	err := DB.Create(e).Error
	if err == nil {
		InvalidateGlobalAccessListCache()
	}
	return err
}

func DeleteGlobalWhitelist(id int) error {
	err := DB.Delete(&GlobalAccessWhitelist{}, id).Error
	if err == nil {
		InvalidateGlobalAccessListCache()
	}
	return err
}

func DeleteGlobalBlacklist(id int) error {
	err := DB.Delete(&GlobalAccessBlacklist{}, id).Error
	if err == nil {
		InvalidateGlobalAccessListCache()
	}
	return err
}

func UpdateGlobalAccessMode(mode string) error {
	err := UpdateOption("GlobalAccessListMode", mode)
	if err == nil {
		InvalidateGlobalAccessListCache()
	}
	return err
}
