package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pennypickbackend/internal/model"
)

// parseDate 解析 YYYY-MM-DD。
func parseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, time.Local)
}

// csvEscape CSV 字段转义：含逗号/引号/换行时加引号包裹。
func csvEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + s + `"`
	}
	return s
}

// BuildBillsCSV 生成账单 CSV 内容（含 UTF-8 BOM），供 HTTP 导出与桌面端保存复用。
// start / end 支持 YYYY-MM-DD 或 YYYY-MM；typ 可选 expense/income。
func BuildBillsCSV(db *gorm.DB, userID uint, start, end, typ string) (string, error) {
	q := db.Model(&model.Bill{}).Where("user_id = ?", userID)
	if s := strings.TrimSpace(start); s != "" {
		if len(s) == 7 {
			if t, _, ok := monthRange(s); ok {
				q = q.Where("occurred_at >= ?", t)
			}
		} else if t, err := parseDate(s); err == nil {
			q = q.Where("occurred_at >= ?", t)
		}
	}
	if e := strings.TrimSpace(end); e != "" {
		if len(e) == 7 {
			if _, t, ok := monthRange(e); ok {
				q = q.Where("occurred_at < ?", t)
			}
		} else if t, err := parseDate(e); err == nil {
			q = q.Where("occurred_at < ?", t.AddDate(0, 0, 1))
		}
	}
	if typ == model.TypeExpense || typ == model.TypeIncome {
		q = q.Where("type = ?", typ)
	}

	var bills []model.Bill
	if err := q.Preload("Category").Preload("Account").Preload("Tags").Order("occurred_at asc, id asc").Find(&bills).Error; err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	sb.WriteString("日期,类型,分类,账户,金额,退款,标签,备注\n")
	for _, b := range bills {
		typName := "支出"
		if b.Type == model.TypeIncome {
			typName = "收入"
		}
		catName, accName := "", ""
		if b.Category != nil {
			catName = b.Category.Name
		}
		if b.Account != nil {
			accName = b.Account.Name
		}
		tagNames := make([]string, 0, len(b.Tags))
		for _, t := range b.Tags {
			tagNames = append(tagNames, t.Name)
		}
		row := fmt.Sprintf("%s,%s,%s,%s,%.2f,%.2f,%s,%s\n",
			b.OccurredAt.Time.Format("2006-01-02 15:04"),
			typName,
			csvEscape(catName),
			csvEscape(accName),
			b.Amount,
			b.RefundAmount,
			csvEscape(strings.Join(tagNames, "|")),
			csvEscape(b.Note),
		)
		sb.WriteString(row)
	}
	return sb.String(), nil
}

// ExportBills 导出账单 CSV（UTF-8 BOM，Excel 可直接打开）。
// 参数：start / end 支持 YYYY-MM-DD 或 YYYY-MM；type 可选 expense/income。
func (h *Handler) ExportBills(c *gin.Context) {
	cu := currentUser(c)
	content, err := BuildBillsCSV(h.db, cu.ID, c.Query("start"), c.Query("end"), c.Query("type"))
	if err != nil {
		fail(c, 500, "导出失败")
		return
	}
	start := c.Query("start")
	end := c.Query("end")
	rangeStr := start + "_" + end
	if rangeStr == "_" || rangeStr == "" {
		rangeStr = "all"
	}
	filename := fmt.Sprintf("pennypick_bills_%s.csv", rangeStr)

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Access-Control-Expose-Headers", "Content-Disposition")
	c.Status(http.StatusOK)
	c.Writer.WriteString(content)
}
