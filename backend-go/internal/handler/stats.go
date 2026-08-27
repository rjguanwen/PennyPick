package handler

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// nowMonth 当前月份 YYYY-MM。
func nowMonth() string {
	return time.Now().Format("2006-01")
}

// monthRange 返回月份区间 [start, end)，ok=false 表示格式错误。
func monthRange(month string) (time.Time, time.Time, bool) {
	t, err := time.ParseInLocation("2006-01", strings.TrimSpace(month), time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return t, t.AddDate(0, 1, 0), true
}

// BudgetInfo 预算进度信息。
type BudgetInfo struct {
	Amount      float64 `json:"amount"`
	WarnPercent float64 `json:"warn_percent"`
	UsedPercent float64 `json:"used_percent"`
	Status      string  `json:"status"` // none / normal / warning / exceeded
	Set         bool    `json:"set"`    // 是否已设置预算
}

// Overview 月度概览：收支汇总 + 预算预警。
func (h *Handler) Overview(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	start, end, ok := monthRange(month)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var bills []model.Bill
	h.db.Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, start, end).Find(&bills)

	var expense, income float64
	for _, b := range bills {
		if b.Type == model.TypeExpense {
			expense += b.NetAmount() // 支出按净额（扣减退款）
		} else {
			income += b.Amount
		}
	}
	expense, income = model.Round2(expense), model.Round2(income)

	var budgetInfo *BudgetInfo
	var budget model.Budget
	if err := h.db.Where("user_id = ? AND month = ?", cu.ID, month).First(&budget).Error; err == nil {
		bi := &BudgetInfo{Amount: budget.Amount, WarnPercent: budget.WarnPercent, Status: "normal", Set: true}
		if budget.Amount > 0 {
			bi.UsedPercent = math.Round(expense/budget.Amount*1000) / 10
			switch {
			case bi.UsedPercent >= 100:
				bi.Status = "exceeded"
			case bi.UsedPercent >= budget.WarnPercent:
				bi.Status = "warning"
			}
		} else {
			bi.Status = "none"
		}
		budgetInfo = bi
	}

	c.JSON(200, gin.H{
		"month":         month,
		"expense_total": expense,
		"income_total":  income,
		"balance":       model.Round2(income - expense),
		"bill_count":    len(bills),
		"budget":        budgetInfo,
	})
}

// CategoryStat 分类维度统计。
type CategoryStat struct {
	CategoryID uint    `json:"category_id"`
	Name       string  `json:"name"`
	Icon       string  `json:"icon"`
	Color      string  `json:"color"`
	Total      float64 `json:"total"`
	Percent    float64 `json:"percent"`
}

// ByCategory 分类统计（某月某类型）。
func (h *Handler) ByCategory(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	typ := c.DefaultQuery("type", model.TypeExpense)
	start, end, ok := monthRange(month)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var bills []model.Bill
	h.db.Preload("Category").
		Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, typ, start, end).
		Find(&bills)

	agg := map[uint]*CategoryStat{}
	var total float64
	for _, b := range bills {
		if b.Category == nil {
			continue
		}
		s, exists := agg[b.CategoryID]
		if !exists {
			s = &CategoryStat{
				CategoryID: b.CategoryID,
				Name:       b.Category.Name,
				Icon:       b.Category.Icon,
				Color:      b.Category.Color,
			}
			agg[b.CategoryID] = s
		}
		s.Total += b.NetAmount() // 分类支出按净额（扣减退款）
		total += b.NetAmount()
	}
	list := make([]CategoryStat, 0, len(agg))
	for _, s := range agg {
		if total > 0 {
			s.Percent = math.Round(s.Total/total*1000) / 10
		}
		list = append(list, *s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Total > list[j].Total })
	c.JSON(200, list)
}

// TrendPoint 趋势点。
type TrendPoint struct {
	Label   string  `json:"label"`
	Expense float64 `json:"expense"`
	Income  float64 `json:"income"`
}

// Trend 收支趋势：granularity=month（start/end 为 YYYY-MM）或 day（YYYY-MM-DD）。
func (h *Handler) Trend(c *gin.Context) {
	cu := currentUser(c)
	granularity := c.DefaultQuery("granularity", "month")

	var start, end time.Time
	var ok bool
	switch granularity {
	case "day":
		start, end, ok = parseDayRange(strings.TrimSpace(c.Query("start")), strings.TrimSpace(c.Query("end")))
	default:
		granularity = "month"
		s := strings.TrimSpace(c.DefaultQuery("start", nowMonth()))
		e := strings.TrimSpace(c.DefaultQuery("end", nowMonth()))
		start, end, ok = monthRange(s)
		if ok {
			var endT time.Time
			endT, _, ok = monthRange(e)
			if ok {
				end = endT.AddDate(0, 1, 0)
			}
		}
	}
	if !ok {
		badRequest(c, "时间范围格式不正确")
		return
	}

	var bills []model.Bill
	h.db.Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, start, end).Find(&bills)

	byLabel := map[string]*TrendPoint{}
	for _, b := range bills {
		var label string
		if granularity == "day" {
			label = b.OccurredAt.Time.Format("01-02")
		} else {
			label = b.OccurredAt.Time.Format("2006-01")
		}
		p, exists := byLabel[label]
		if !exists {
			p = &TrendPoint{Label: label}
			byLabel[label] = p
		}
		if b.Type == model.TypeExpense {
			p.Expense += b.NetAmount() // 支出按净额（扣减退款）
		} else {
			p.Income += b.Amount
		}
	}

	points := make([]TrendPoint, 0)
	for cur := start; cur.Before(end); {
		var label string
		if granularity == "day" {
			label = cur.Format("01-02")
			cur = cur.AddDate(0, 0, 1)
		} else {
			label = cur.Format("2006-01")
			cur = cur.AddDate(0, 1, 0)
		}
		p, exists := byLabel[label]
		if !exists {
			points = append(points, TrendPoint{Label: label})
			continue
		}
		points = append(points, *p)
	}
	c.JSON(200, points)
}

func parseDayRange(startStr, endStr string) (time.Time, time.Time, bool) {
	if startStr == "" || endStr == "" {
		return time.Time{}, time.Time{}, false
	}
	start, err1 := time.ParseInLocation("2006-01-02", startStr, time.Local)
	end, err2 := time.ParseInLocation("2006-01-02", endStr, time.Local)
	if err1 != nil || err2 != nil || end.Before(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end.AddDate(0, 0, 1), true
}

// AccountStat 账户维度统计。
type AccountStat struct {
	AccountID uint    `json:"account_id"`
	Name      string  `json:"name"`
	Icon      string  `json:"icon"`
	Expense   float64 `json:"expense"`
	Income    float64 `json:"income"`
}

// TagStat 标签维度统计。
type TagStat struct {
	TagID     uint    `json:"tag_id"`
	Name      string  `json:"name"`
	Total     float64 `json:"total"`
	Percent   float64 `json:"percent"`
	BillCount int     `json:"bill_count"`
}

// Tags 按标签统计（某月某类型）。
func (h *Handler) Tags(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	typ := c.DefaultQuery("type", model.TypeExpense)
	start, end, ok := monthRange(month)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var bills []model.Bill
	h.db.Preload("Tags").
		Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, typ, start, end).
		Find(&bills)

	agg := map[uint]*TagStat{}
	var total float64
	for _, b := range bills {
		for _, t := range b.Tags {
			s, exists := agg[t.ID]
			if !exists {
				s = &TagStat{TagID: t.ID, Name: t.Name}
				agg[t.ID] = s
			}
			s.Total += b.Amount
			s.BillCount++
			total += b.Amount
		}
	}
	list := make([]TagStat, 0, len(agg))
	for _, s := range agg {
		if total > 0 {
			s.Percent = math.Round(s.Total/total*1000) / 10
		}
		list = append(list, *s)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Total > list[j].Total })
	c.JSON(200, list)
}

// AccountStats 账户收支统计。
func (h *Handler) AccountStats(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	start, end, ok := monthRange(month)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var bills []model.Bill
	h.db.Preload("Account").
		Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, start, end).
		Find(&bills)

	agg := map[uint]*AccountStat{}
	for _, b := range bills {
		if b.Account == nil {
			continue
		}
		aid := *b.AccountID
		s, exists := agg[aid]
		if !exists {
			s = &AccountStat{AccountID: aid, Name: b.Account.Name, Icon: b.Account.Icon}
			agg[aid] = s
		}
		if b.Type == model.TypeExpense {
			s.Expense += b.NetAmount() // 账户支出按净额（扣减退款）
		} else {
			s.Income += b.Amount
		}
	}
	list := make([]AccountStat, 0, len(agg))
	for _, s := range agg {
		s.Expense = model.Round2(s.Expense)
		s.Income = model.Round2(s.Income)
		list = append(list, *s)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Expense+list[i].Income > list[j].Expense+list[j].Income
	})
	c.JSON(200, list)
}
