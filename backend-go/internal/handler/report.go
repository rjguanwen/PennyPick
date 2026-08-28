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

	// 保证数组字段不为 null（前端模板容错）
	if data.Accounts == nil {
		data.Accounts = []ReportAccount{}
	}

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
	start, end, ok := monthRange(req.Month)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	// 先判断该月是否有账单：无账单则直接提示，不尝试生成
	var cnt int64
	h.db.Model(&model.Bill{}).Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, start, end).Count(&cnt)
	if cnt == 0 {
		badRequest(c, "本月没有账单，无法生成报告")
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

// ---------- 年度报告 ----------

// YearlyMonthPoint 某月收支点。
type YearlyMonthPoint struct {
	Month   string  `json:"month"`
	Expense float64 `json:"expense"`
	Income  float64 `json:"income"`
}

// YearlyAccountTrend 某账户年内按月收支（所有账户放到一张图）。
type YearlyAccountTrend struct {
	AccountID uint               `json:"account_id"`
	Name      string             `json:"name"`
	Icon      string             `json:"icon"`
	IsCredit  bool               `json:"is_credit"`
	Months    []YearlyMonthPoint `json:"months"`
}

// YearlyAccountSummary 年度账户收支汇总。
type YearlyAccountSummary struct {
	AccountID uint    `json:"account_id"`
	Name      string  `json:"name"`
	Icon      string  `json:"icon"`
	Expense   float64 `json:"expense"`
	Income    float64 `json:"income"`
}

// YearlyCategory 年度分类支出汇总。
type YearlyCategory struct {
	CategoryID uint    `json:"category_id"`
	Name       string  `json:"name"`
	Icon       string  `json:"icon"`
	Color      string  `json:"color"`
	Total      float64 `json:"total"`
	Percent    float64 `json:"percent"`
}

// YearlyRepayPoint 信用账户某月还款情况。
type YearlyRepayPoint struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"` // full 全额 / partial 部分 / "" 未还款
}

// YearlyCreditRepay 信用账户年内还款汇总。
type YearlyCreditRepay struct {
	AccountID uint               `json:"account_id"`
	Name      string             `json:"name"`
	Total     float64            `json:"total"`
	Months    []YearlyRepayPoint `json:"months"`
}

// YearlyReportData 年度报告完整数据。
type YearlyReportData struct {
	Year           int                    `json:"year"`
	ExpenseTotal   float64                `json:"expense_total"`
	IncomeTotal    float64                `json:"income_total"`
	Balance        float64                `json:"balance"`
	BillCount      int                    `json:"bill_count"`
	RefundTotal    float64                `json:"refund_total"`
	MonthlyTrend   []YearlyMonthPoint     `json:"monthly_trend"`
	AccountTrend   []YearlyAccountTrend   `json:"account_trend"`
	AccountSummary []YearlyAccountSummary `json:"account_summary"`
	Categories     []YearlyCategory       `json:"categories"`
	Tags           []ReportTag            `json:"tags"`
	CreditRepay    []YearlyCreditRepay    `json:"credit_repayment"`
}

// GenerateYearlyReport 生成（或覆盖）年度收支报告并保存，供列表后续查看。
func (h *Handler) GenerateYearlyReport(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Year string `json:"year"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Year) == "" {
		badRequest(c, "请提供年份")
		return
	}
	year, err := strconv.Atoi(strings.TrimSpace(req.Year))
	if err != nil || year < 2000 || year > 2200 {
		badRequest(c, "年份不正确")
		return
	}
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(1, 0, 0)

	// 先判断该年是否有账单：无账单则直接提示，不尝试生成
	var cnt int64
	h.db.Model(&model.Bill{}).Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, start, end).Count(&cnt)
	if cnt == 0 {
		badRequest(c, "本年没有账单，无法生成报告")
		return
	}

	// 全年账单（含分类/账户/标签）
	var bills []model.Bill
	h.db.Preload("Category").Preload("Account").Preload("Tags").
		Where("user_id = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, start, end).
		Order("occurred_at asc, id asc").Find(&bills)

	data := &YearlyReportData{Year: year}

	// 概览 + 月度趋势（初始化 12 月）
	monthIdx := map[string]int{}
	for i := 0; i < 12; i++ {
		m := time.Date(year, time.Month(i+1), 1, 0, 0, 0, 0, time.Local).Format("2006-01")
		monthIdx[m] = i
		data.MonthlyTrend = append(data.MonthlyTrend, YearlyMonthPoint{Month: m})
	}
	for _, b := range bills {
		idx, ok := monthIdx[b.OccurredAt.Time.Format("2006-01")]
		if !ok {
			continue
		}
		if b.Type == model.TypeExpense {
			net := b.Amount - b.RefundAmount
			data.MonthlyTrend[idx].Expense += net
			data.ExpenseTotal += net
			data.RefundTotal += b.RefundAmount
		} else {
			data.MonthlyTrend[idx].Income += b.Amount
			data.IncomeTotal += b.Amount
		}
		data.BillCount++
	}
	data.ExpenseTotal = model.Round2(data.ExpenseTotal)
	data.IncomeTotal = model.Round2(data.IncomeTotal)
	data.Balance = model.Round2(data.IncomeTotal - data.ExpenseTotal)
	data.RefundTotal = model.Round2(data.RefundTotal)
	for i := range data.MonthlyTrend {
		data.MonthlyTrend[i].Expense = model.Round2(data.MonthlyTrend[i].Expense)
		data.MonthlyTrend[i].Income = model.Round2(data.MonthlyTrend[i].Income)
	}

	// 账户：趋势 + 汇总（包含"无账户"项，保证未选账户的账单也完整计入分析）
	var accounts []model.Account
	h.db.Where("user_id = ?", cu.ID).Order("sort_order asc, id asc").Find(&accounts)
	accIdx := map[uint]int{}
	buildAcc := func(aid uint, name string) {
		accIdx[aid] = len(data.AccountTrend)
		at := YearlyAccountTrend{AccountID: aid, Name: name}
		for j := 0; j < 12; j++ {
			at.Months = append(at.Months, YearlyMonthPoint{Month: data.MonthlyTrend[j].Month})
		}
		data.AccountTrend = append(data.AccountTrend, at)
		data.AccountSummary = append(data.AccountSummary, YearlyAccountSummary{AccountID: aid, Name: name})
	}
	for _, a := range accounts {
		buildAcc(a.ID, a.Name)
	}
	buildAcc(0, "无账户")
	for _, b := range bills {
		aid := uint(0)
		if b.AccountID != nil {
			aid = *b.AccountID
		}
		ai, ok := accIdx[aid]
		if !ok {
			continue
		}
		mi, ok2 := monthIdx[b.OccurredAt.Time.Format("2006-01")]
		if !ok2 {
			continue
		}
		if b.Type == model.TypeExpense {
			net := b.Amount - b.RefundAmount
			data.AccountTrend[ai].Months[mi].Expense += net
			data.AccountSummary[ai].Expense += net
		} else {
			data.AccountTrend[ai].Months[mi].Income += b.Amount
			data.AccountSummary[ai].Income += b.Amount
		}
	}
	// 仅保留年内有收支的账户
	keptTrend := data.AccountTrend[:0]
	keptSum := data.AccountSummary[:0]
	for i := range data.AccountTrend {
		has := false
		for j := range data.AccountTrend[i].Months {
			data.AccountTrend[i].Months[j].Expense = model.Round2(data.AccountTrend[i].Months[j].Expense)
			data.AccountTrend[i].Months[j].Income = model.Round2(data.AccountTrend[i].Months[j].Income)
			if data.AccountTrend[i].Months[j].Expense > 0 || data.AccountTrend[i].Months[j].Income > 0 {
				has = true
			}
		}
		if has {
			keptTrend = append(keptTrend, data.AccountTrend[i])
			data.AccountSummary[i].Expense = model.Round2(data.AccountSummary[i].Expense)
			data.AccountSummary[i].Income = model.Round2(data.AccountSummary[i].Income)
			keptSum = append(keptSum, data.AccountSummary[i])
		}
	}
	data.AccountTrend = keptTrend
	data.AccountSummary = keptSum
	sort.Slice(data.AccountSummary, func(i, j int) bool {
		return data.AccountSummary[i].Expense+data.AccountSummary[i].Income > data.AccountSummary[j].Expense+data.AccountSummary[j].Income
	})

	// 分类支出汇总
	var cats []model.Category
	h.db.Where("user_id = ?", cu.ID).Find(&cats)
	catMap := map[uint]model.Category{}
	for _, c := range cats {
		catMap[c.ID] = c
	}
	catTotal := map[uint]float64{}
	for _, b := range bills {
		if b.Type == model.TypeExpense {
			catTotal[b.CategoryID] += b.Amount - b.RefundAmount
		}
	}
	for cid, total := range catTotal {
		total = model.Round2(total)
		c := catMap[cid]
		item := YearlyCategory{CategoryID: cid, Name: c.Name, Icon: c.Icon, Color: c.Color, Total: total}
		if data.ExpenseTotal > 0 {
			item.Percent = math.Round(total/data.ExpenseTotal*1000) / 10
		}
		data.Categories = append(data.Categories, item)
	}
	sort.Slice(data.Categories, func(i, j int) bool { return data.Categories[i].Total > data.Categories[j].Total })

	// 支出标签汇总
	tagAgg := map[uint]*ReportTag{}
	for _, b := range bills {
		if b.Type != model.TypeExpense {
			continue
		}
		for _, t := range b.Tags {
			item, exists := tagAgg[t.ID]
			if !exists {
				item = &ReportTag{TagID: t.ID, Name: t.Name}
				tagAgg[t.ID] = item
			}
			item.Total += b.NetAmount()
			item.Count++
		}
	}
	for _, t := range tagAgg {
		t.Total = model.Round2(t.Total)
		data.Tags = append(data.Tags, *t)
	}
	sort.Slice(data.Tags, func(i, j int) bool { return data.Tags[i].Total > data.Tags[j].Total })

	// 信用账户年内各月还款情况
	var creditAccts []model.Account
	h.db.Where("user_id = ? AND is_credit = ?", cu.ID, true).Order("sort_order asc, id asc").Find(&creditAccts)
	if len(creditAccts) > 0 {
		ids := make([]uint, 0, len(creditAccts))
		creditIdx := map[uint]int{}
		for i, a := range creditAccts {
			ids = append(ids, a.ID)
			creditIdx[a.ID] = i
			cr := YearlyCreditRepay{AccountID: a.ID, Name: a.Name}
			for j := 0; j < 12; j++ {
				cr.Months = append(cr.Months, YearlyRepayPoint{Month: data.MonthlyTrend[j].Month})
			}
			data.CreditRepay = append(data.CreditRepay, cr)
		}
		var reps []model.Repayment
		h.db.Where("user_id = ? AND account_id IN ? AND month LIKE ?", cu.ID, ids, strconv.Itoa(year)+"-%").Find(&reps)
		for _, r := range reps {
			ci, ok := creditIdx[r.AccountID]
			if !ok {
				continue
			}
			mi, ok2 := monthIdx[r.Month]
			if !ok2 {
				continue
			}
			data.CreditRepay[ci].Months[mi].Amount = model.Round2(data.CreditRepay[ci].Months[mi].Amount + r.Amount)
			data.CreditRepay[ci].Total += r.Amount
			switch r.Status {
			case model.RepayStatusFull:
				data.CreditRepay[ci].Months[mi].Status = model.RepayStatusFull
			case model.RepayStatusPartial:
				if data.CreditRepay[ci].Months[mi].Status != model.RepayStatusFull {
					data.CreditRepay[ci].Months[mi].Status = model.RepayStatusPartial
				}
			}
		}
		for i := range data.CreditRepay {
			data.CreditRepay[i].Total = model.Round2(data.CreditRepay[i].Total)
			for j := range data.CreditRepay[i].Months {
				data.CreditRepay[i].Months[j].Amount = model.Round2(data.CreditRepay[i].Months[j].Amount)
			}
		}
	}

	// 保证数组字段不为 null（前端模板容错）
	if data.Tags == nil {
		data.Tags = []ReportTag{}
	}
	if data.Categories == nil {
		data.Categories = []YearlyCategory{}
	}
	if data.CreditRepay == nil {
		data.CreditRepay = []YearlyCreditRepay{}
	}
	if data.AccountTrend == nil {
		data.AccountTrend = []YearlyAccountTrend{}
	}
	if data.AccountSummary == nil {
		data.AccountSummary = []YearlyAccountSummary{}
	}

	// upsert：同一年份只保留一份，重新生成即覆盖
	payload, _ := json.Marshal(data)
	var rep model.YearlyReport
	if err := h.db.Where("user_id = ? AND year = ?", cu.ID, strconv.Itoa(year)).First(&rep).Error; err == nil {
		rep.ExpenseTotal = data.ExpenseTotal
		rep.IncomeTotal = data.IncomeTotal
		rep.Data = string(payload)
		rep.CreatedAt = time.Now()
		h.db.Save(&rep)
	} else {
		rep = model.YearlyReport{
			UserID:       cu.ID,
			Year:         strconv.Itoa(year),
			ExpenseTotal: data.ExpenseTotal,
			IncomeTotal:  data.IncomeTotal,
			Data:         string(payload),
			CreatedAt:    time.Now(),
		}
		h.db.Create(&rep)
	}
	c.JSON(200, gin.H{"id": rep.ID, "year": rep.Year, "created_at": rep.CreatedAt, "data": data})
}

// ListYearlyReports 年度报告列表。
func (h *Handler) ListYearlyReports(c *gin.Context) {
	cu := currentUser(c)
	var reps []model.YearlyReport
	h.db.Where("user_id = ?", cu.ID).Order("year desc, created_at desc").Find(&reps)
	list := make([]gin.H, 0, len(reps))
	for _, r := range reps {
		list = append(list, gin.H{
			"id":            r.ID,
			"year":          r.Year,
			"expense_total": r.ExpenseTotal,
			"income_total":  r.IncomeTotal,
			"created_at":    r.CreatedAt,
		})
	}
	c.JSON(200, list)
}

// GetYearlyReport 年度报告详情。
func (h *Handler) GetYearlyReport(c *gin.Context) {
	cu := currentUser(c)
	var rep model.YearlyReport
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), cu.ID).First(&rep).Error; err != nil {
		notFound(c, "报告不存在")
		return
	}
	var data YearlyReportData
	if err := json.Unmarshal([]byte(rep.Data), &data); err != nil {
		fail(c, 500, "报告数据解析失败")
		return
	}
	c.JSON(200, gin.H{"id": rep.ID, "year": rep.Year, "created_at": rep.CreatedAt, "data": data})
}
