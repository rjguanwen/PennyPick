package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// accountMonthRange 返回某账户某展示月的统计区间。
// 信用账户按账期（上月出账日至本月出账日）；非信用或未设出账日按自然月。
func accountMonthRange(acc *model.Account, month string) (time.Time, time.Time) {
	if !acc.IsCredit || acc.StatementDay <= 0 {
		start, end, _ := monthRange(month)
		return start, end
	}
	day := acc.StatementDay
	if day > 28 {
		day = 28
	}
	t, _ := time.ParseInLocation("2006-01", month, time.Local)
	thisStart := time.Date(t.Year(), t.Month(), day, 0, 0, 0, 0, time.Local)
	return thisStart.AddDate(0, -1, 0), thisStart
}

// AccountQueryMonth 账户某月支出。
type AccountQueryMonth struct {
	Month   string  `json:"month"`
	Expense float64 `json:"expense"`
}

// AccountQueryRow 账户查询行。
type AccountQueryRow struct {
	AccountID    uint                `json:"account_id"`
	Name         string              `json:"name"`
	Icon         string              `json:"icon"`
	IsCredit     bool                `json:"is_credit"`
	StatementDay int                 `json:"statement_day"`
	Months       []AccountQueryMonth `json:"months"`
}

// AccountQueryResult 账户查询结果。
type AccountQueryResult struct {
	Start          string            `json:"start"`
	End            string            `json:"end"`
	Months         []string          `json:"months"`
	CreditAccounts []AccountQueryRow `json:"credit_accounts"`
	NormalAccounts []AccountQueryRow `json:"normal_accounts"`
}

// AccountQuery 查询时间段内各账户每月支出（信用账户按账期，非信用按自然月）。
func (h *Handler) AccountQuery(c *gin.Context) {
	cu := currentUser(c)
	now := time.Now()
	start := strings.TrimSpace(c.DefaultQuery("start", time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local).Format("2006-01")))
	end := strings.TrimSpace(c.DefaultQuery("end", now.Format("2006-01")))
	startT, _, ok := monthRange(start)
	if !ok {
		badRequest(c, "开始月份格式不正确")
		return
	}
	endT, _, ok := monthRange(end)
	if !ok {
		badRequest(c, "结束月份格式不正确")
		return
	}
	if endT.Before(startT) {
		badRequest(c, "结束月份不能早于开始月份")
		return
	}
	// 月份列表（跨度最多 12 个月）
	months := []string{}
	for t := startT; !t.After(endT); t = t.AddDate(0, 1, 0) {
		months = append(months, t.Format("2006-01"))
	}
	if len(months) > 12 {
		badRequest(c, "查询月份跨度不能超过 12 个月")
		return
	}

	// 账户
	var accounts []model.Account
	h.db.Where("user_id = ?", cu.ID).Order("sort_order asc, id asc").Find(&accounts)
	accMap := map[uint]*model.Account{}
	for i := range accounts {
		accMap[accounts[i].ID] = &accounts[i]
	}

	// 区间内支出账单
	// 信用账户展示月 M 的账期 = [M-1 出账日, M 出账日)，开始月份的账期起点可能早于 startT（早在上月末），
	// 因此查询起点需提前一个月，确保账期内属于上月末的账单被完整统计；归属仍按逐月账期/自然月判断，不影响其他月份。
	rangeStart := startT.AddDate(0, -1, 0)
	rangeEnd := endT.AddDate(0, 1, 0)
	var bills []model.Bill
	h.db.Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?",
		cu.ID, model.TypeExpense, rangeStart, rangeEnd).Find(&bills)

	// 账户 × 月份支出矩阵
	matrix := map[uint]map[string]float64{}
	for _, acc := range accounts {
		matrix[acc.ID] = map[string]float64{}
		for _, m := range months {
			matrix[acc.ID][m] = 0
		}
	}
	for _, b := range bills {
		if b.AccountID == nil {
			continue
		}
		acc := accMap[*b.AccountID]
		if acc == nil {
			continue
		}
		net := b.Amount - b.RefundAmount
		for _, m := range months {
			rs, re := accountMonthRange(acc, m)
			if !b.OccurredAt.Time.Before(rs) && b.OccurredAt.Time.Before(re) {
				matrix[acc.ID][m] += net
				break
			}
		}
	}

	result := &AccountQueryResult{Start: start, End: end, Months: months}
	for _, acc := range accounts {
		row := AccountQueryRow{
			AccountID:    acc.ID,
			Name:         acc.Name,
			Icon:         acc.Icon,
			IsCredit:     acc.IsCredit,
			StatementDay: acc.StatementDay,
		}
		for _, m := range months {
			row.Months = append(row.Months, AccountQueryMonth{Month: m, Expense: model.Round2(matrix[acc.ID][m])})
		}
		if acc.IsCredit {
			result.CreditAccounts = append(result.CreditAccounts, row)
		} else {
			result.NormalAccounts = append(result.NormalAccounts, row)
		}
	}
	c.JSON(200, result)
}

// AccountQueryBillItem 账户某月支出明细项。
type AccountQueryBillItem struct {
	ID           uint        `json:"id"`
	OccurredAt   string      `json:"occurred_at"`
	CategoryName string      `json:"category_name"`
	CategoryIcon string      `json:"category_icon"`
	Amount       float64     `json:"amount"`
	Note         string      `json:"note"`
	Tags         []model.Tag `json:"tags"`
}

// AccountQueryBills 某账户某展示月的支出明细（账期/自然月口径与统计一致）。
func (h *Handler) AccountQueryBills(c *gin.Context) {
	cu := currentUser(c)
	accountID, err := strconv.ParseUint(c.Query("account_id"), 10, 64)
	month := strings.TrimSpace(c.Query("month"))
	if err != nil || accountID == 0 || month == "" {
		badRequest(c, "缺少账户或月份参数")
		return
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ?", accountID, cu.ID).First(&acc).Error; err != nil {
		notFound(c, "账户不存在")
		return
	}
	rs, re := accountMonthRange(&acc, month)
	var bills []model.Bill
	h.db.Preload("Category").Preload("Tags").
		Where("user_id = ? AND account_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?",
			cu.ID, accountID, model.TypeExpense, rs, re).
		Order("occurred_at asc, id asc").Find(&bills)
	items := make([]AccountQueryBillItem, 0, len(bills))
	for _, b := range bills {
		catName, catIcon := "", ""
		if b.Category != nil {
			catName = b.Category.Name
			catIcon = b.Category.Icon
		}
		items = append(items, AccountQueryBillItem{
			ID:           b.ID,
			OccurredAt:   b.OccurredAt.Time.Format("2006-01-02 15:04"),
			CategoryName: catName,
			CategoryIcon: catIcon,
			Amount:       model.Round2(b.Amount - b.RefundAmount),
			Note:         b.Note,
			Tags:         b.Tags,
		})
	}
	c.JSON(200, gin.H{"account_id": acc.ID, "account_name": acc.Name, "month": month, "items": items})
}
