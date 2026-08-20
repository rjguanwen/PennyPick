package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

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

// ExportBills 导出账单 CSV（UTF-8 BOM，Excel 可直接打开）。
// 参数：start / end 支持 YYYY-MM-DD 或 YYYY-MM；type 可选 expense/income。
func (h *Handler) ExportBills(c *gin.Context) {
	cu := currentUser(c)

	q := h.db.Model(&model.Bill{}).Where("user_id = ?", cu.ID)
	if s := strings.TrimSpace(c.Query("start")); s != "" {
		if len(s) == 7 {
			if t, _, ok := monthRange(s); ok {
				q = q.Where("occurred_at >= ?", t)
			}
		} else if t, err := parseDate(s); err == nil {
			q = q.Where("occurred_at >= ?", t)
		}
	}
	if e := strings.TrimSpace(c.Query("end")); e != "" {
		if len(e) == 7 {
			if _, t, ok := monthRange(e); ok {
				q = q.Where("occurred_at < ?", t)
			}
		} else if t, err := parseDate(e); err == nil {
			q = q.Where("occurred_at < ?", t.AddDate(0, 0, 1))
		}
	}
	if typ := c.Query("type"); typ == model.TypeExpense || typ == model.TypeIncome {
		q = q.Where("type = ?", typ)
	}

	var bills []model.Bill
	if err := q.Preload("Category").Preload("Account").Order("occurred_at asc, id asc").Find(&bills).Error; err != nil {
		fail(c, 500, "导出失败")
		return
	}

	var sb strings.Builder
	sb.WriteString("\xEF\xBB\xBF") // UTF-8 BOM
	sb.WriteString("日期,类型,分类,账户,金额,备注\n")
	for _, b := range bills {
		typ := "支出"
		if b.Type == model.TypeIncome {
			typ = "收入"
		}
		catName, accName := "", ""
		if b.Category != nil {
			catName = b.Category.Name
		}
		if b.Account != nil {
			accName = b.Account.Name
		}
		row := fmt.Sprintf("%s,%s,%s,%s,%.2f,%s\n",
			b.OccurredAt.Time.Format("2006-01-02 15:04"),
			typ,
			csvEscape(catName),
			csvEscape(accName),
			b.Amount,
			csvEscape(b.Note),
		)
		sb.WriteString(row)
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
	c.Writer.WriteString(sb.String())
}
