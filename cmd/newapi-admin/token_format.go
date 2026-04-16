package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

func formatUnixLocal(sec int64) string {
	if sec <= 0 {
		return "-"
	}
	return time.Unix(sec, 0).In(time.Local).Format(timeLayoutDateTime)
}

func formatExpiredLocal(sec int64) string {
	if sec < 0 {
		return "永不过期"
	}
	return formatUnixLocal(sec)
}

func parseAPISuccessData(b []byte) (json.RawMessage, error) {
	var w struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}
	if !w.Success {
		msg := strings.TrimSpace(w.Message)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return w.Data, nil
}

type adminTokenRow struct {
	ID                 int    `json:"id"`
	UserID             int    `json:"user_id"`
	Username           string `json:"username"`
	Name               string `json:"name"`
	Key                string `json:"key"`
	Status             int    `json:"status"`
	Group              string `json:"group"`
	CreatedTime        int64  `json:"created_time"`
	AccessedTime       int64  `json:"accessed_time"`
	ExpiredTime        int64  `json:"expired_time"`
	RemainQuota        int    `json:"remain_quota"`
	UsedQuota          int    `json:"used_quota"`
	UnlimitedQuota     bool   `json:"unlimited_quota"`
	ModelLimitsEnabled bool   `json:"model_limits_enabled"`
}

type tokenListPayload struct {
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int             `json:"total"`
	Items    []adminTokenRow `json:"items"`
}

type tokenListDisplayRow struct {
	ID           int    `json:"id"`
	UserID       int    `json:"user_id"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Key          string `json:"key"`
	Status       int    `json:"status"`
	Group        string `json:"group"`
	CreatedTime  string `json:"created_time"`
	AccessedTime string `json:"accessed_time"`
	ExpiredTime  string `json:"expired_time"`
	RemainQuota  int    `json:"remain_quota"`
	UsedQuota    int    `json:"used_quota"`
}

func toTokenListDisplayRows(p *tokenListPayload) []tokenListDisplayRow {
	rows := make([]tokenListDisplayRow, 0, len(p.Items))
	for _, it := range p.Items {
		rows = append(rows, tokenListDisplayRow{
			ID:           it.ID,
			UserID:       it.UserID,
			Username:     it.Username,
			Name:         it.Name,
			Key:          it.Key,
			Status:       it.Status,
			Group:        it.Group,
			CreatedTime:  formatUnixLocal(it.CreatedTime),
			AccessedTime: formatUnixLocal(it.AccessedTime),
			ExpiredTime:  formatExpiredLocal(it.ExpiredTime),
			RemainQuota:  it.RemainQuota,
			UsedQuota:    it.UsedQuota,
		})
	}
	return rows
}

func printTokenListTable(w io.Writer, p *tokenListPayload) {
	table := tablewriter.NewWriter(w)
	table.Header([]string{"ID", "用户ID", "用户名", "名称", "密钥", "状态", "分组", "创建时间", "访问时间", "过期", "剩余额度", "已用额度"})
	for _, row := range toTokenListDisplayRows(p) {
		table.Append([]string{
			fmt.Sprintf("%d", row.ID),
			fmt.Sprintf("%d", row.UserID),
			row.Username,
			row.Name,
			row.Key,
			fmt.Sprintf("%d", row.Status),
			row.Group,
			row.CreatedTime,
			row.AccessedTime,
			row.ExpiredTime,
			fmt.Sprintf("%d", row.RemainQuota),
			fmt.Sprintf("%d", row.UsedQuota),
		})
	}
	table.Render()
	fmt.Fprintf(w, "共 %d 条，第 %d 页，每页 %d 条\n", p.Total, p.Page, p.PageSize)
}

func printTokenListCSV(w io.Writer, p *tokenListPayload) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "user_id", "username", "name", "key", "status", "group", "created_time", "accessed_time", "expired_time", "remain_quota", "used_quota"}); err != nil {
		return err
	}
	for _, row := range toTokenListDisplayRows(p) {
		if err := cw.Write([]string{
			fmt.Sprintf("%d", row.ID),
			fmt.Sprintf("%d", row.UserID),
			row.Username,
			row.Name,
			row.Key,
			fmt.Sprintf("%d", row.Status),
			row.Group,
			row.CreatedTime,
			row.AccessedTime,
			row.ExpiredTime,
			fmt.Sprintf("%d", row.RemainQuota),
			fmt.Sprintf("%d", row.UsedQuota),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func printTokenListJSON(w io.Writer, p *tokenListPayload) error {
	out := struct {
		Page     int                   `json:"page"`
		PageSize int                   `json:"page_size"`
		Total    int                   `json:"total"`
		Items    []tokenListDisplayRow `json:"items"`
	}{
		Page:     p.Page,
		PageSize: p.PageSize,
		Total:    p.Total,
		Items:    toTokenListDisplayRows(p),
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

func printTokenDetailBlock(w io.Writer, username string, t *adminTokenRow) {
	fmt.Fprintf(w, "username: %s\n", username)
	fmt.Fprintf(w, "id: %d\n", t.ID)
	fmt.Fprintf(w, "user_id: %d\n", t.UserID)
	fmt.Fprintf(w, "name: %s\n", t.Name)
	fmt.Fprintf(w, "key: %s\n", t.Key)
	fmt.Fprintf(w, "status: %d\n", t.Status)
	fmt.Fprintf(w, "group: %s\n", t.Group)
	fmt.Fprintf(w, "created_time: %s\n", formatUnixLocal(t.CreatedTime))
	fmt.Fprintf(w, "accessed_time: %s\n", formatUnixLocal(t.AccessedTime))
	fmt.Fprintf(w, "expired_time: %s\n", formatExpiredLocal(t.ExpiredTime))
	fmt.Fprintf(w, "remain_quota: %d\n", t.RemainQuota)
	fmt.Fprintf(w, "used_quota: %d\n", t.UsedQuota)
	fmt.Fprintf(w, "unlimited_quota: %v\n", t.UnlimitedQuota)
	fmt.Fprintf(w, "model_limits_enabled: %v\n", t.ModelLimitsEnabled)
}

type periodRange struct {
	Start int64 `json:"start_timestamp"`
	End   int64 `json:"end_timestamp"`
}

type logBrief struct {
	ID               int    `json:"id"`
	CreatedAt        int64  `json:"created_at"`
	ModelName        string `json:"model_name"`
	Quota            int    `json:"quota"`
	Ip               string `json:"ip"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

type tokenDetailExt struct {
	Username    string     `json:"username"`
	Token       adminTokenRow `json:"token"`
	Period      periodRange   `json:"period"`
	CallCount   int           `json:"call_count"`
	QuotaSum    int           `json:"quota_sum"`
	DistinctIPs []string      `json:"distinct_ips"`
	RecentLogs  []logBrief    `json:"recent_logs"`
}

func printTokenDetailExtended(w io.Writer, d *tokenDetailExt) {
	printTokenDetailBlock(w, d.Username, &d.Token)
	fmt.Fprintln(w, "--- 统计区间 ---")
	fmt.Fprintf(w, "start: %s\n", formatUnixLocal(d.Period.Start))
	fmt.Fprintf(w, "end:   %s\n", formatUnixLocal(d.Period.End))
	fmt.Fprintf(w, "call_count: %d\n", d.CallCount)
	fmt.Fprintf(w, "quota_sum: %d\n", d.QuotaSum)
	fmt.Fprintf(w, "distinct_ips (%d): %s\n", len(d.DistinctIPs), strings.Join(d.DistinctIPs, ", "))
	if len(d.RecentLogs) == 0 {
		fmt.Fprintln(w, "--- 近期日志 ---\n(无)")
		return
	}
	fmt.Fprintln(w, "--- 近期日志 ---")
	tw := tablewriter.NewWriter(w)
	tw.Header([]string{"ID", "时间", "模型", "额度", "IP", "Prompt", "Completion"})
	for _, lg := range d.RecentLogs {
		tw.Append([]string{
			fmt.Sprintf("%d", lg.ID),
			formatUnixLocal(lg.CreatedAt),
			lg.ModelName,
			fmt.Sprintf("%d", lg.Quota),
			lg.Ip,
			fmt.Sprintf("%d", lg.PromptTokens),
			fmt.Sprintf("%d", lg.CompletionTokens),
		})
	}
	tw.Render()
}
