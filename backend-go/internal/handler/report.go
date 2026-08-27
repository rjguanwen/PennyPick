package handler

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// ---------- 报告数据结构 ----------

// MonthCompare 与某期的收支对比。
type MonthCompare struct {
	Expense       float64  `json:"expense"`
	Income        float64  `json:"income"`
	ExpenseChange *float64 `json:"expense_change_pct"` // 变化率（%），上期为 0 时为 nil
	IncomeChange  *float64 `json:"income_change_pct"`
}

// ReportOverview 月度概览。
type ReportOverview struct {
	Month        string        `json:"month"`
	ExpenseTotal float64       `json:"expense_total"`
	IncomeTotal  float64       `json:"income_total"`
	Balance      float64       `json:"balance"`
	BillCount    int           `json:"bill_count"`
	DailyAvg     float64       `json:"daily_avg"`
	PrevMonth    *MonthCompare `json:"prev_month"`
	LastYear     *MonthCompare `json:"last_year"`
}

// ReportCategory 分类统计（与上月对比）。
type ReportCategory struct {
	CategoryID uint    `json:"category_id"`
	Name       string  `json:"name"`
	Icon       string  `json:"icon"`
	Color      string  `json:"color"`
	Total      float64 `json:"total"`
	Percent    float64 `json:"percent"`
	PrevTotal  float64 `json:"prev_total"`
	ChangePct  *float64 `json:"change_pct"`
}

// ReportTrend 月度趋势点。
type ReportTrend struct {
	Month   string  `json:"month"`
	Expense float64 `json:"expense"`
	Income  float64 `json:"income"`
}

// ReportAccount 账户收支统计。
type ReportAccount struct {
	AccountID uint    `json:"account_id"`
	Name      string  `json:"name"`
	Icon      string  `json:"icon"`
	Expense   float64 `json:"expense"`
	Income    float64 `json:"income"`
}

// ReportTag 标签统计。
type ReportTag struct {
	TagID uint    `json:"tag_id"`
	Name  string  `json:"name"`
	Total float64 `json:"total"`
	Count int     `json:"count"`
}

// ReportData 报告完整数据。
type ReportData struct {
	Month      string           `json:"month"`
	Overview   ReportOverview   `json:"overview"`
	Categories []ReportCategory `json:"categories"`
	Trend      []ReportTrend    `json:"trend"`
	Accounts   []ReportAccount  `json:"accounts"`
	Tags       []ReportTag      `json:"tags"`
}

// ---------- 工具 ----------

func shiftMonthStr(month string, delta int) string {
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return month
	}
	return t.AddDate(0, delta, 0).Format("2006-01")
}

func diffPct(cur, prev float64) *float64 {
	if prev <= 0 {
		return nil
	}
	pct := math.Round((cur-prev)/prev*1000) / 10
	return &pct
}

// monthTotal 某月收支聚合。
func (h *Handler) monthTotal(userID uint, month string) (expense, income float64, count int) {
	start, end, ok := monthRange(month)
	if !ok {
		return 0, 0, 0
	}
	var rows []struct {
		Type         string
		Amount       float64
		RefundAmount float64
	}
	h.db.Model(&model.Bill{}).
		Select("type, amount, refund_amount").
		Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", userID, start, end).
		Scan(&rows)
	for _, r := range rows {
		if r.Type == model.TypeExpense {
			expense += r.Amount - r.RefundAmount // 支出按净额（扣减退款）
		} else if r.Type == model.TypeIncome {
			income += r.Amount
		}
		count++
	}
	return model.Round2(expense), model.Round2(income), count
}

// buildReportData 计算某月的完整报告数据。
func (h *Handler) buildReportData(userID uint, month string) *ReportData {
	start, end, ok := monthRange(month)
	if !ok {
		return nil
	}
	data := &ReportData{Month: month}

	// 1. 概览
	expense, income, count := h.monthTotal(userID, month)
	prevExpense, prevIncome, _ := h.monthTotal(userID, shiftMonthStr(month, -1))
	lyExpense, lyIncome, _ := h.monthTotal(userID, shiftMonthStr(month, -12))
	dailyAvg := 0.0
	if days := int(end.Sub(start).Hours() / 24); days > 0 {
		dailyAvg = model.Round2(expense / float64(days))
	}
	data.Overview = ReportOverview{
		Month:        month,
		ExpenseTotal: expense,
		IncomeTotal:  income,
		Balance:      model.Round2(income - expense),
		BillCount:    count,
		DailyAvg:     dailyAvg,
		PrevMonth: &MonthCompare{
			Expense:       prevExpense,
			Income:        prevIncome,
			ExpenseChange: diffPct(expense, prevExpense),
			IncomeChange:  diffPct(income, prevIncome),
		},
		LastYear: &MonthCompare{
			Expense:       lyExpense,
			Income:        lyIncome,
			ExpenseChange: diffPct(expense, lyExpense),
			IncomeChange:  diffPct(income, lyIncome),
		},
	}

	// 2. 分类统计（本月支出，与上月对比）
	var cats []model.Category
	h.db.Where("user_id = ?", userID).Find(&cats)
	catMap := map[uint]model.Category{}
	for _, c := range cats {
		catMap[c.ID] = c
	}
	curCat := map[uint]float64{}
	var billRows []struct {
		CategoryID   uint
		Amount       float64
		RefundAmount float64
	}
	h.db.Model(&model.Bill{}).
		Select("category_id, amount, refund_amount").
		Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?", userID, model.TypeExpense, start, end).
		Scan(&billRows)
	for _, r := range billRows {
		curCat[r.CategoryID] += r.Amount - r.RefundAmount
	}
	prevStart, prevEnd, _ := monthRange(shiftMonthStr(month, -1))
	prevCat := map[uint]float64{}
	var prevRows []struct {
		CategoryID   uint
		Amount       float64
		RefundAmount float64
	}
	h.db.Model(&model.Bill{}).
		Select("category_id, amount, refund_amount").
		Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?", userID, model.TypeExpense, prevStart, prevEnd).
		Scan(&prevRows)
	for _, r := range prevRows {
		prevCat[r.CategoryID] += r.Amount - r.RefundAmount
	}
	data.Categories = make([]ReportCategory, 0, len(curCat))
	for cid, total := range curCat {
		total = model.Round2(total)
		c := catMap[cid]
		item := ReportCategory{
			CategoryID: cid,
			Name:       c.Name,
			Icon:       c.Icon,
			Color:      c.Color,
			Total:      total,
			Percent:    0,
			PrevTotal:  model.Round2(prevCat[cid]),
			ChangePct:  diffPct(total, prevCat[cid]),
		}
		data.Categories = append(data.Categories, item)
	}
	if expense > 0 {
		for i := range data.Categories {
			data.Categories[i].Percent = math.Round(data.Categories[i].Total/expense*1000) / 10
		}
	}
	sort.Slice(data.Categories, func(i, j int) bool { return data.Categories[i].Total > data.Categories[j].Total })

	// 3. 近 6 个月趋势
	for i := 5; i >= 0; i-- {
		m := shiftMonthStr(month, -i)
		e, inc, _ := h.monthTotal(userID, m)
		data.Trend = append(data.Trend, ReportTrend{Month: m, Expense: e, Income: inc})
	}

	// 4. 账户收支
	var accounts []model.Account
	h.db.Where("user_id = ?", userID).Order("sort_order asc, id asc").Find(&accounts)
	accExp := map[uint]float64{}
	accInc := map[uint]float64{}
	var accRows []struct {
		AccountID    *uint
		Type         string
		Amount       float64
		RefundAmount float64
	}
	h.db.Model(&model.Bill{}).
		Select("account_id, type, amount, refund_amount").
		Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", userID, start, end).
		Scan(&accRows)
	for _, r := range accRows {
		if r.AccountID == nil {
			continue
		}
		if r.Type == model.TypeExpense {
			accExp[*r.AccountID] += r.Amount - r.RefundAmount
		} else {
			accInc[*r.AccountID] += r.Amount
		}
	}
	for _, a := range accounts {
		if accExp[a.ID] == 0 && accInc[a.ID] == 0 {
			continue
		}
		data.Accounts = append(data.Accounts, ReportAccount{
			AccountID: a.ID,
			Name:      a.Name,
			Icon:      a.Icon,
			Expense:   model.Round2(accExp[a.ID]),
			Income:    model.Round2(accInc[a.ID]),
		})
	}
	sort.Slice(data.Accounts, func(i, j int) bool {
		return data.Accounts[i].Expense+data.Accounts[i].Income > data.Accounts[j].Expense+data.Accounts[j].Income
	})

	// 5. 标签统计
	var bills []model.Bill
	h.db.Preload("Tags").
		Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?", userID, model.TypeExpense, start, end).
		Find(&bills)
	tagAgg := map[uint]*ReportTag{}
	for _, b := range bills {
		for _, t := range b.Tags {
			item, exists := tagAgg[t.ID]
			if !exists {
				item = &ReportTag{TagID: t.ID, Name: t.Name}
				tagAgg[t.ID] = item
			}
			item.Total += b.NetAmount() // 标签支出按净额（扣减退款）
			item.Count++
		}
	}
	data.Tags = make([]ReportTag, 0, len(tagAgg))
	for _, t := range tagAgg {
		t.Total = model.Round2(t.Total)
		data.Tags = append(data.Tags, *t)
	}
	sort.Slice(data.Tags, func(i, j int) bool { return data.Tags[i].Total > data.Tags[j].Total })

	return data
}

// ---------- Handler ----------

// GenerateReport 生成（覆盖）某月报告。
func (h *Handler) GenerateReport(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Month string `json:"month"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.Month = strings.TrimSpace(req.Month)
	if _, _, ok := monthRange(req.Month); !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	data := h.buildReportData(cu.ID, req.Month)
	if data == nil {
		badRequest(c, "月份格式不正确")
		return
	}
	raw, err := json.Marshal(data)
	if err != nil {
		fail(c, 500, "报告生成失败")
		return
	}
	var rep model.MonthlyReport
	h.db.Where("user_id = ? AND month = ?", cu.ID, req.Month).Limit(1).Find(&rep)
	if rep.ID > 0 {
		rep.Data = string(raw)
		rep.ExpenseTotal = data.Overview.ExpenseTotal
		rep.IncomeTotal = data.Overview.IncomeTotal
		if err := h.db.Save(&rep).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	} else {
		rep = model.MonthlyReport{
			UserID:       cu.ID,
			Month:        req.Month,
			ExpenseTotal: data.Overview.ExpenseTotal,
			IncomeTotal:  data.Overview.IncomeTotal,
			Data:         string(raw),
		}
		if err := h.db.Create(&rep).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	}
	c.JSON(200, gin.H{"id": rep.ID, "month": rep.Month, "created_at": rep.CreatedAt, "data": data})
}

// ListReports 已生成报告列表（按月份倒序）。
func (h *Handler) ListReports(c *gin.Context) {
	cu := currentUser(c)
	var list []model.MonthlyReport
	if err := h.db.Where("user_id = ?", cu.ID).
		Order("month desc, id desc").Find(&list).Error; err != nil {
		fail(c, 500, "查询失败")
		return
	}
	c.JSON(200, list)
}

// GetReport 查看报告详情。
func (h *Handler) GetReport(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "报告不存在")
		return
	}
	var rep model.MonthlyReport
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&rep).Error; err != nil {
		notFound(c, "报告不存在")
		return
	}
	var data ReportData
	if err := json.Unmarshal([]byte(rep.Data), &data); err != nil {
		fail(c, 500, "报告数据解析失败")
		return
	}
	c.JSON(200, gin.H{"id": rep.ID, "month": rep.Month, "created_at": rep.CreatedAt, "data": data})
}
