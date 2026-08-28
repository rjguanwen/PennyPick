package handler

import (
	"math"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pennypickbackend/internal/model"
)

// GetBudget 查询指定月份预算。
func (h *Handler) GetBudget(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	var b model.Budget
	err := h.db.Where("user_id = ? AND month = ?", cu.ID, month).First(&b).Error
	if err != nil {
		c.JSON(200, gin.H{"month": month, "amount": 0, "warn_percent": 80, "set": false})
		return
	}
	c.JSON(200, gin.H{"month": b.Month, "amount": b.Amount, "warn_percent": b.WarnPercent, "set": true})
}

// ListBudgets 全部月度预算（按月倒序）。
func (h *Handler) ListBudgets(c *gin.Context) {
	cu := currentUser(c)
	var budgets []model.Budget
	if err := h.db.Where("user_id = ?", cu.ID).Order("month desc").Find(&budgets).Error; err != nil {
		fail(c, 500, "查询预算失败")
		return
	}
	c.JSON(200, budgets)
}

// UpsertBudget 设置/更新某月预算。
func (h *Handler) UpsertBudget(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Month       string  `json:"month"`
		Amount      float64 `json:"amount"`
		WarnPercent float64 `json:"warn_percent"`
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
	if req.Amount < 0 || req.Amount > 999999999 {
		badRequest(c, "预算金额不正确")
		return
	}
	if req.WarnPercent <= 0 {
		req.WarnPercent = 80
	}
	if req.WarnPercent > 100 {
		req.WarnPercent = 100
	}

	var b model.Budget
	err := h.db.Where("user_id = ? AND month = ?", cu.ID, req.Month).First(&b).Error
	if err != nil {
		b = model.Budget{UserID: cu.ID, Month: req.Month, Amount: model.Round2(req.Amount), WarnPercent: req.WarnPercent}
		if err := h.db.Create(&b).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	} else {
		b.Amount = model.Round2(req.Amount)
		b.WarnPercent = req.WarnPercent
		if err := h.db.Save(&b).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	}
	c.JSON(200, gin.H{"month": b.Month, "amount": b.Amount, "warn_percent": b.WarnPercent, "set": true})
}

// DeleteBudget 删除某月预算。
func (h *Handler) DeleteBudget(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.Query("month"))
	if month == "" {
		badRequest(c, "缺少月份参数")
		return
	}
	h.db.Where("user_id = ? AND month = ?", cu.ID, month).Delete(&model.Budget{})
	c.JSON(200, gin.H{"ok": true})
}

// categoryBudgetStatus 计算已用占比与预警状态。
func categoryBudgetStatus(used, amount, warnPercent float64) (float64, string) {
	usedPercent := math.Round(used/amount*1000) / 10
	status := "normal"
	if usedPercent >= 100 {
		status = "exceeded"
	} else if usedPercent >= warnPercent {
		status = "warning"
	}
	return usedPercent, status
}

// ListCategoryBudgets 某月各支出分类的预算与已用情况。
func (h *Handler) ListCategoryBudgets(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.DefaultQuery("month", nowMonth()))
	if _, _, ok := monthRange(month); !ok {
		badRequest(c, "月份格式不正确")
		return
	}

	// 该用户全部支出分类
	var cats []model.Category
	if err := h.db.Where("user_id = ? AND type = ?", cu.ID, model.TypeExpense).
		Order("sort_order asc, id asc").Find(&cats).Error; err != nil {
		fail(c, 500, "查询分类失败")
		return
	}

	// 该月分类预算
	var cbs []model.CategoryBudget
	h.db.Where("user_id = ? AND month = ?", cu.ID, month).Find(&cbs)
	cbMap := map[uint]model.CategoryBudget{}
	for _, cb := range cbs {
		cbMap[cb.CategoryID] = cb
	}

	// 该月各分类支出汇总
	start, end, _ := monthRange(month)
	var rows []struct {
		CategoryID uint
		Total      float64
	}
	h.db.Model(&model.Bill{}).
		Select("category_id, COALESCE(SUM(amount - refund_amount), 0) as total").
		Where("user_id = ? AND type = ? AND occurred_at >= ? AND occurred_at < ?", cu.ID, model.TypeExpense, start, end).
		Group("category_id").Scan(&rows)
	usedMap := map[uint]float64{}
	for _, r := range rows {
		usedMap[r.CategoryID] = model.Round2(r.Total)
	}

	list := make([]gin.H, 0, len(cats))
	for _, cat := range cats {
		used := usedMap[cat.ID]
		item := gin.H{"category": cat, "used": used, "budget": nil}
		if cb, ok := cbMap[cat.ID]; ok && cb.Amount > 0 {
			usedPercent, status := categoryBudgetStatus(used, cb.Amount, cb.WarnPercent)
			item["budget"] = gin.H{
				"amount":       cb.Amount,
				"warn_percent": cb.WarnPercent,
				"used_percent": usedPercent,
				"status":       status,
			}
		}
		list = append(list, item)
	}
	c.JSON(200, list)
}

// UpsertCategoryBudget 设置/更新某月某分类的预算。
func (h *Handler) UpsertCategoryBudget(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Month       string  `json:"month"`
		CategoryID  uint    `json:"category_id"`
		Amount      float64 `json:"amount"`
		WarnPercent float64 `json:"warn_percent"`
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
	if req.Amount <= 0 || req.Amount > 999999999 {
		badRequest(c, "预算金额需大于 0")
		return
	}
	if req.WarnPercent <= 0 {
		req.WarnPercent = 80
	}
	if req.WarnPercent > 100 {
		req.WarnPercent = 100
	}
	// 校验分类归属与类型
	var cat model.Category
	if err := h.db.Where("id = ? AND user_id = ? AND type = ?", req.CategoryID, cu.ID, model.TypeExpense).First(&cat).Error; err != nil {
		badRequest(c, "分类不存在或不是支出分类")
		return
	}

	var cb model.CategoryBudget
	err := h.db.Where("user_id = ? AND month = ? AND category_id = ?", cu.ID, req.Month, req.CategoryID).First(&cb).Error
	if err != nil {
		cb = model.CategoryBudget{
			UserID:      cu.ID,
			Month:       req.Month,
			CategoryID:  req.CategoryID,
			Amount:      model.Round2(req.Amount),
			WarnPercent: req.WarnPercent,
		}
		if err := h.db.Create(&cb).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	} else {
		cb.Amount = model.Round2(req.Amount)
		cb.WarnPercent = req.WarnPercent
		if err := h.db.Save(&cb).Error; err != nil {
			fail(c, 500, "保存失败")
			return
		}
	}
	c.JSON(200, gin.H{"month": cb.Month, "category_id": cb.CategoryID, "amount": cb.Amount, "warn_percent": cb.WarnPercent})
}

// DeleteCategoryBudget 删除某月某分类的预算。
func (h *Handler) DeleteCategoryBudget(c *gin.Context) {
	cu := currentUser(c)
	month := strings.TrimSpace(c.Query("month"))
	catID, err := strconv.ParseUint(c.Query("category_id"), 10, 64)
	if month == "" || err != nil {
		badRequest(c, "缺少月份或分类参数")
		return
	}
	h.db.Where("user_id = ? AND month = ? AND category_id = ?", cu.ID, month, catID).Delete(&model.CategoryBudget{})
	c.JSON(200, gin.H{"ok": true})
}

// CopyBudget 将源月份预算（总预算 + 分类预算）复制到目标月份，覆盖目标月已有预算。
func (h *Handler) CopyBudget(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		FromMonth string `json:"from_month"`
		ToMonth   string `json:"to_month"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.FromMonth = strings.TrimSpace(req.FromMonth)
	req.ToMonth = strings.TrimSpace(req.ToMonth)
	if _, _, ok := monthRange(req.FromMonth); !ok {
		badRequest(c, "源月份格式不正确")
		return
	}
	if _, _, ok := monthRange(req.ToMonth); !ok {
		badRequest(c, "目标月份格式不正确")
		return
	}
	if req.FromMonth == req.ToMonth {
		badRequest(c, "源月份与目标月份不能相同")
		return
	}

	// 读取源月预算
	var src model.Budget
	h.db.Where("user_id = ? AND month = ?", cu.ID, req.FromMonth).First(&src)
	var srcCbs []model.CategoryBudget
	h.db.Where("user_id = ? AND month = ?", cu.ID, req.FromMonth).Find(&srcCbs)
	if src.ID == 0 && len(srcCbs) == 0 {
		badRequest(c, "源月份没有可复制的预算")
		return
	}

	// 事务写入目标月（总预算 upsert，分类预算先清后写，实现整体覆盖）
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if src.ID != 0 {
			var dst model.Budget
			if err := tx.Where("user_id = ? AND month = ?", cu.ID, req.ToMonth).First(&dst).Error; err == nil {
				dst.Amount = src.Amount
				dst.WarnPercent = src.WarnPercent
				if err := tx.Save(&dst).Error; err != nil {
					return err
				}
			} else {
				dst = model.Budget{UserID: cu.ID, Month: req.ToMonth, Amount: src.Amount, WarnPercent: src.WarnPercent}
				if err := tx.Create(&dst).Error; err != nil {
					return err
				}
			}
		}
		// 分类预算：目标月清空后写入
		if err := tx.Where("user_id = ? AND month = ?", cu.ID, req.ToMonth).Delete(&model.CategoryBudget{}).Error; err != nil {
			return err
		}
		for _, cb := range srcCbs {
			dst := model.CategoryBudget{
				UserID:      cu.ID,
				Month:       req.ToMonth,
				CategoryID:  cb.CategoryID,
				Amount:      cb.Amount,
				WarnPercent: cb.WarnPercent,
			}
			if err := tx.Create(&dst).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		fail(c, 500, "复制预算失败")
		return
	}
	c.JSON(200, gin.H{"total_copied": src.ID != 0, "category_count": len(srcCbs)})
}
