package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// RepaymentItem 单个信用账户的还款状态。
type RepaymentItem struct {
	Account      *model.Account `json:"account"`
	Repaid       bool           `json:"repaid"`
	RepaidAt     *time.Time     `json:"repaid_at"`
	Note         string         `json:"note"`
	DueDay       int            `json:"due_day"`
	Overdue      bool           `json:"overdue"`
	OverdueBy    int            `json:"overdue_by"` // 逾期天数
	MonthExpense float64        `json:"month_expense"` // 本月该账户支出总额
	HasExpense   bool           `json:"has_expense"`   // 本月该账户是否有支出
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

	// 该月各账户支出总额（仅支出类型，用于判断是否需要还款）
	expenseMap := map[uint]float64{}
	if len(accounts) > 0 {
		start, end, _ := monthRange(month)
		ids := make([]uint, 0, len(accounts))
		for _, a := range accounts {
			ids = append(ids, a.ID)
		}
		var rows []struct {
			AccountID uint
			Total     float64
		}
		h.db.Model(&model.Bill{}).
			Select("account_id, COALESCE(SUM(amount), 0) as total").
			Where("user_id = ? AND type = ? AND account_id IN ? AND occurred_at >= ? AND occurred_at < ?",
				cu.ID, model.TypeExpense, ids, start, end).
			Group("account_id").Scan(&rows)
		for _, r := range rows {
			expenseMap[r.AccountID] = r.Total
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
		expense := model.Round2(expenseMap[acc.ID])
		item := RepaymentItem{
			Account:      &acc,
			DueDay:       acc.RepayDay,
			MonthExpense: expense,
			HasExpense:   expense > 0,
		}
		if r, ok := repMap[acc.ID]; ok {
			item.Repaid = true
			item.RepaidAt = &r.CreatedAt
			item.Note = r.Note
		} else if item.HasExpense && month == todayMonth && acc.RepayDay > 0 && today.Day() > acc.RepayDay {
			// 仅当月且有支出时判定逾期
			item.Overdue = true
			item.OverdueBy = today.Day() - acc.RepayDay
		}
		items = append(items, item)
	}
	c.JSON(200, items)
}

// MarkRepayment 标记某账户某月已还款（幂等：重复标记仅更新备注）。
func (h *Handler) MarkRepayment(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		AccountID uint   `json:"account_id"`
		Month     string `json:"month"`
		Note      string `json:"note"`
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
	note := strings.TrimSpace(req.Note)
	var existing model.Repayment
	if err := h.db.Where("user_id = ? AND account_id = ? AND month = ?", cu.ID, acc.ID, req.Month).
		First(&existing).Error; err == nil {
		existing.Note = note
		if err := h.db.Save(&existing).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
		c.JSON(200, existing)
		return
	}
	rep := &model.Repayment{
		UserID:    cu.ID,
		AccountID: acc.ID,
		Month:     req.Month,
		Note:      note,
	}
	if err := h.db.Create(rep).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	c.JSON(201, rep)
}

// UnmarkRepayment 取消某账户某月的还款标记。
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
	if err := h.db.Where("user_id = ? AND account_id = ? AND month = ?", cu.ID, accountID, month).
		Delete(&model.Repayment{}).Error; err != nil {
		fail(c, 500, "取消失败")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
