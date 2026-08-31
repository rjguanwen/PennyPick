package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// diffBillNote 补差账单的固定备注。
const diffBillNote = "补差：小额消费合计"

// RepaymentItem 单个信用账户的还款状态。
type RepaymentItem struct {
	Account        *model.Account `json:"account"`
	Repaid         bool           `json:"repaid"`
	RepaidAt       *time.Time     `json:"repaid_at"`
	Note           string         `json:"note"`
	Amount         float64        `json:"amount"`         // 实际还款金额
	Status         string         `json:"status"`         // full 全额 / partial 部分
	NeedsReconfirm bool           `json:"needs_reconfirm"` // 已标记全额还款但实际还款额 < 当前应还金额（账期内补录了新账单），需重新确认
	DueDay         int            `json:"due_day"`
	Overdue        bool           `json:"overdue"`
	OverdueBy      int            `json:"overdue_by"`    // 逾期天数
	MonthExpense   float64        `json:"month_expense"` // 本期账单应还金额（账期内支出总额）
	HasExpense     bool           `json:"has_expense"`   // 本期是否有支出
	BillingStart   string         `json:"billing_start"` // 本期账期开始（YYYY-MM-DD）
	BillingEnd     string         `json:"billing_end"`   // 本期账期结束（YYYY-MM-DD，不含）
}

// billingRange 返回查看月份 month（YYYY-MM）对应的账期 [start, end)。
// 规则：本月还款对应的是「本月出账日」生成的账单（账期 = 上月出账日 ~ 本月出账日）；
// 但当还款日早于出账日（还款日前本期账单尚未出账）时，本月还的是上月出账的账单，账期整体前移一个月。
// statementDay 未设置（<=0 或非法）时按自然月。
func billingRange(month string, statementDay, repayDay int) (time.Time, time.Time, bool) {
	t, err := time.ParseInLocation("2006-01", month, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	first := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
	if statementDay < 1 || statementDay > 28 {
		return first, first.AddDate(0, 1, 0), true // 自然月账期
	}
	if repayDay > 0 && repayDay < statementDay {
		// 还款日在出账日前：本期账单未出，本月还上期账单
		first = first.AddDate(0, -1, 0)
	}
	prevFirst := first.AddDate(0, -1, 0)
	start := time.Date(prevFirst.Year(), prevFirst.Month(), statementDay, 0, 0, 0, 0, time.Local)
	end := time.Date(first.Year(), first.Month(), statementDay, 0, 0, 0, 0, time.Local)
	return start, end, true
}

// ListRepayments 某月各信用账户的还款状态（按 sort_order 排序）。
func (h *Handler) ListRepayments(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	if _, _, ok := monthRange(month); !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var accounts []model.Account
	h.db.Where("user_id = ? AND is_credit = ?", cu.ID, true).
		Order("sort_order asc, id asc").Find(&accounts)

	// 各账户本期账期内的支出总额（按各自出账日，用于判断是否需要还款）
	expenseMap := map[uint]float64{}
	if len(accounts) > 0 {
		for _, a := range accounts {
			start, end, ok := billingRange(month, a.StatementDay, a.RepayDay)
			if !ok {
				continue
			}
			var total float64
			h.db.Model(&model.Bill{}).
				Where("user_id = ? AND type = ? AND account_id = ? AND occurred_at >= ? AND occurred_at < ?",
					cu.ID, model.TypeExpense, a.ID, start, end).
				Select("COALESCE(SUM(amount), 0)").Scan(&total)
			expenseMap[a.ID] = total
		}
	}

	var reps []model.Repayment
	h.db.Where("user_id = ? AND month = ?", cu.ID, month).Find(&reps)
	repMap := map[uint]model.Repayment{}
	for _, r := range reps {
		repMap[r.AccountID] = r
	}
	today := time.Now()
	todayMonth := today.Format("2006-01")
	items := make([]RepaymentItem, 0, len(accounts))
	for _, acc := range accounts {
		bStart, bEnd, _ := billingRange(month, acc.StatementDay, acc.RepayDay)
		expense := model.Round2(expenseMap[acc.ID])
		item := RepaymentItem{
			Account:      &acc,
			DueDay:       acc.RepayDay,
			MonthExpense: expense,
			HasExpense:   expense > 0,
			BillingStart: bStart.Format("2006-01-02"),
			BillingEnd:   bEnd.Format("2006-01-02"),
		}
		if r, ok := repMap[acc.ID]; ok {
			item.Repaid = true
			item.RepaidAt = &r.CreatedAt
			item.Note = r.Note
			item.Amount = r.Amount
			item.Status = r.Status
			if item.Status == "" {
				item.Status = model.RepayStatusFull
			}
			// 已标记全额还款，但当前应还金额超过实际还款额 → 账期内补录了新账单，需重新确认
			if item.Status == model.RepayStatusFull && model.Round2(r.Amount) < expense {
				item.NeedsReconfirm = true
			}
		} else if item.HasExpense && month == todayMonth && acc.RepayDay > 0 && today.Day() > acc.RepayDay {
			// 仅当月且有支出时判定逾期（超过本月还款日）
			item.Overdue = true
			item.OverdueBy = today.Day() - acc.RepayDay
		}
		items = append(items, item)
	}
	c.JSON(200, items)
}

// MarkRepayment 标记某账户某月已还款（幂等）。
// 需录入实际还款金额 amount：
//   - amount > 本期应还（账期内支出合计）：自动补录一张「其他」分类的差额账单（备注：补差：小额消费合计），使账单合计与实际还款一致；
//   - amount < 本期应还：标记为部分还款（partial）；
//   - amount == 本期应还（或未录入）：标记为全额还款（full）。
func (h *Handler) MarkRepayment(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		AccountID uint    `json:"account_id"`
		Month     string  `json:"month"`
		Amount    float64 `json:"amount"`
		Note      string  `json:"note"`
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
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ? AND is_credit = ?", req.AccountID, cu.ID, true).
		First(&acc).Error; err != nil {
		badRequest(c, "账户不存在或非信用账户")
		return
	}

	start, end, ok := billingRange(req.Month, acc.StatementDay, acc.RepayDay)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	// 本期应还金额（账期内支出合计）
	var expenseTotal float64
	h.db.Model(&model.Bill{}).
		Where("user_id = ? AND account_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?",
			cu.ID, acc.ID, model.TypeExpense, start, end).
		Select("COALESCE(SUM(amount), 0)").Scan(&expenseTotal)
	expenseTotal = model.Round2(expenseTotal)

	amount := model.Round2(req.Amount)
	if amount < 0 {
		amount = 0
	}
	note := strings.TrimSpace(req.Note)
	status := model.RepayStatusFull
	diffAmount := 0.0

	// 先清除该账期内已有的补差账单，保证重复标记/修改金额时结果一致
	h.db.Where("user_id = ? AND account_id = ? AND type = ? AND note = ? AND occurred_at >= ? AND occurred_at < ?",
		cu.ID, acc.ID, model.TypeExpense, diffBillNote, start, end).
		Delete(&model.Bill{})

	if amount > expenseTotal {
		// 实际还款大于本期应还：补差账单（日期取账期内最后一天）
		diffAmount = model.Round2(amount - expenseTotal)
		cat, err := h.otherCategory(cu.ID)
		if err != nil {
			fail(c, 500, "未找到「其他」分类")
			return
		}
		diffDate := end.AddDate(0, 0, -1)
		diffBill := &model.Bill{
			UserID:     cu.ID,
			AccountID:  &acc.ID,
			CategoryID: cat.ID,
			Type:       model.TypeExpense,
			Amount:     diffAmount,
			Note:       diffBillNote,
			OccurredAt: model.DateTime{Time: diffDate},
		}
		if err := h.db.Create(diffBill).Error; err != nil {
			fail(c, 500, "补差账单创建失败")
			return
		}
	} else if amount > 0 && amount < expenseTotal {
		// 部分还款
		status = model.RepayStatusPartial
	}

	var existing model.Repayment
	// 用 Find 判断是否已存在，避免 First 在记录不存在时产生 ErrRecordNotFound 日志
	h.db.Where("user_id = ? AND account_id = ? AND month = ?", cu.ID, acc.ID, req.Month).
		Limit(1).Find(&existing)
	if existing.ID > 0 {
		existing.Amount = amount
		existing.Status = status
		existing.Note = note
		if err := h.db.Save(&existing).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	} else {
		rep := &model.Repayment{
			UserID:    cu.ID,
			AccountID: acc.ID,
			Month:     req.Month,
			Amount:    amount,
			Status:    status,
			Note:      note,
		}
		if err := h.db.Create(rep).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	}
	c.JSON(200, gin.H{
		"ok":            true,
		"status":        status,
		"amount":        amount,
		"diff_bill":     diffAmount > 0,
		"diff_amount":   diffAmount,
		"month_expense": model.Round2(expenseTotal + diffAmount),
		"billing_start": start.Format("2006-01-02"),
		"billing_end":   end.Format("2006-01-02"),
	})
}

// UnmarkRepayment 取消某账户某月的还款标记。
// 同时移除该账期内的补差账单，避免数据残留。
func (h *Handler) UnmarkRepayment(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.Query("month"))
	accountID := strings.TrimSpace(c.Query("account_id"))
	if month == "" || accountID == "" {
		badRequest(c, "参数缺失")
		return
	}
	if _, _, ok := monthRange(month); !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ?", accountID, cu.ID).First(&acc).Error; err != nil {
		notFound(c, "账户不存在")
		return
	}
	if err := h.db.Where("user_id = ? AND account_id = ? AND month = ?", cu.ID, accountID, month).
		Delete(&model.Repayment{}).Error; err != nil {
		fail(c, 500, "取消失败")
		return
	}
	// 清理该账期补差账单
	start, end, ok := billingRange(month, acc.StatementDay, acc.RepayDay)
	if ok {
		h.db.Where("user_id = ? AND account_id = ? AND type = ? AND note = ? AND occurred_at >= ? AND occurred_at < ?",
			cu.ID, accountID, model.TypeExpense, diffBillNote, start, end).
			Delete(&model.Bill{})
	}
	c.JSON(200, gin.H{"ok": true})
}

// ListRepaymentBills 某账户在指定月份账期内的账单明细（用于还款页查看账期应还组成）。
func (h *Handler) ListRepaymentBills(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.Query("month"))
	accountID := strings.TrimSpace(c.Query("account_id"))
	if month == "" || accountID == "" {
		badRequest(c, "参数缺失")
		return
	}
	if _, _, ok := monthRange(month); !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ? AND is_credit = ?", accountID, cu.ID, true).
		First(&acc).Error; err != nil {
		badRequest(c, "账户不存在或非信用账户")
		return
	}
	start, end, ok := billingRange(month, acc.StatementDay, acc.RepayDay)
	if !ok {
		badRequest(c, "月份格式不正确")
		return
	}
	var bills []model.Bill
	h.db.Preload("Category").Preload("Account").Preload("Tags").
		Where("user_id = ? AND account_id = ? AND occurred_at >= ? AND occurred_at < ?",
			cu.ID, acc.ID, start, end).
		Order("occurred_at asc, id asc").Find(&bills)
	var expense, income float64
	for _, b := range bills {
		if b.Type == model.TypeExpense {
			expense += b.Amount
		} else {
			income += b.Amount
		}
	}
	c.JSON(200, gin.H{
		"billing_start": start.Format("2006-01-02"),
		"billing_end":   end.Format("2006-01-02"),
		"expense_total": model.Round2(expense),
		"income_total":  model.Round2(income),
		"items":         bills,
	})
}

// otherCategory 获取用户支出类型下的「其他」固定分类。
func (h *Handler) otherCategory(userID uint) (*model.Category, error) {
	var cat model.Category
	if err := h.db.Where("user_id = ? AND type = ? AND name = ?", userID, model.TypeExpense, fixedCategoryName).
		First(&cat).Error; err != nil {
		return nil, err
	}
	return &cat, nil
}

// repaymentHit 一次“账单落入已标记还款账期”的命中信息。
type repaymentHit struct {
	AccountName string
	Month       string
	Amount      float64
}

// repaymentHitLabel 生成「账户名·M月账期」提示片段。
func (r *repaymentHit) label() string {
	if r == nil || r.AccountName == "" {
		return ""
	}
	return "「" + r.AccountName + "」" + r.Month + "账期"
}

// repaymentMarkedFor 检查一笔账单（账户 + 日期 + 类型）是否落入某个已标记还款的账期。
// 命中返回命中信息，未命中返回 nil。
// 说明：账单日期所在账期对应的还款月份只会晚于或等于账单所在自然月（账期起点总是早于账单日期），
// 因此只需检查 occurredAt 所在月及其后两个月即可覆盖所有账期映射（含还款日早于出账日的跨期场景）。
func (h *Handler) repaymentMarkedFor(userID, accountID uint, occurredAt time.Time, typ string) *repaymentHit {
	if typ != model.TypeExpense || occurredAt.IsZero() {
		return nil
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ?", accountID, userID).First(&acc).Error; err != nil {
		return nil
	}
	if !acc.IsCredit {
		return nil
	}
	for i := 0; i < 3; i++ {
		m := occurredAt.AddDate(0, i, 0).Format("2006-01")
		start, end, ok := billingRange(m, acc.StatementDay, acc.RepayDay)
		if !ok {
			continue
		}
		if !occurredAt.Before(start) && occurredAt.Before(end) {
			var rep model.Repayment
			h.db.Where("user_id = ? AND account_id = ? AND month = ?", userID, accountID, m).Limit(1).Find(&rep)
			if rep.ID > 0 {
				return &repaymentHit{AccountName: acc.Name, Month: m, Amount: rep.Amount}
			}
			return nil
		}
	}
	return nil
}

// repaymentWarnText 生成单笔账单的还款提醒文案（未命中返回空串）。
func repaymentWarnText(hit *repaymentHit) string {
	if hit == nil {
		return ""
	}
	return fmt.Sprintf("「%s」在 %s 已标记还款（还款 ¥%.2f），该账单落在其账期内，录入后请到「账户还款」页重新确认还款情况",
		hit.AccountName, hit.Month, hit.Amount)
}

// batchRepaymentWarnText 批量录入场景：把命中的「账户·月份」聚合成提示文案。
func batchRepaymentWarnText(hits []*repaymentHit) string {
	seen := map[string]bool{}
	var names []string
	for _, h := range hits {
		if h == nil {
			continue
		}
		if lbl := h.label(); lbl != "" && !seen[lbl] {
			seen[lbl] = true
			names = append(names, lbl)
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "以下账期已标记还款：" + strings.Join(names, "、") + "，请到「账户还款」页重新确认还款情况"
}
