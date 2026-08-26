package handler

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pennypickbackend/internal/model"
)

type recurringReq struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	CategoryID uint    `json:"category_id"`
	AccountID  *uint   `json:"account_id"`
	Amount     float64 `json:"amount"`
	Day        int     `json:"day"`
	Note       string  `json:"note"`
	TagIDs     []uint  `json:"tag_ids"`
	Active     *bool   `json:"active"`
}

func (r *recurringReq) validate() (bool, string) {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || utf8.RuneCountInString(r.Name) > 64 {
		return false, "名称需为 1-64 个字符"
	}
	if r.Type != model.TypeExpense && r.Type != model.TypeIncome {
		return false, "类型不正确"
	}
	if r.CategoryID == 0 {
		return false, "请选择分类"
	}
	if r.Amount <= 0 {
		return false, "金额需大于 0"
	}
	if r.Day < 1 || r.Day > 28 {
		r.Day = 1
	}
	return true, ""
}

// ListRecurringBills 固定账单模板列表。
func (h *Handler) ListRecurringBills(c *gin.Context) {
	cu := currentUser(c)
	var list []model.RecurringBill
	if err := h.db.Where("user_id = ?", cu.ID).
		Order("active desc, id asc").Find(&list).Error; err != nil {
		fail(c, 500, "查询失败")
		return
	}
	c.JSON(200, list)
}

// CreateRecurringBill 新建固定账单模板。
func (h *Handler) CreateRecurringBill(c *gin.Context) {
	cu := currentUser(c)
	var req recurringReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if ok, msg := req.validate(); !ok {
		badRequest(c, msg)
		return
	}
	if !h.checkCategory(h.db, cu.ID, req.CategoryID, req.Type) {
		badRequest(c, "分类不存在或与账单类型不匹配")
		return
	}
	if req.AccountID != nil {
		var cnt int64
		h.db.Model(&model.Account{}).Where("id = ? AND user_id = ?", *req.AccountID, cu.ID).Count(&cnt)
		if cnt == 0 {
			badRequest(c, "账户不存在")
			return
		}
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	rb := &model.RecurringBill{
		UserID:     cu.ID,
		Name:       req.Name,
		Type:       req.Type,
		CategoryID: req.CategoryID,
		AccountID:  req.AccountID,
		Amount:     model.Round2(req.Amount),
		Day:        req.Day,
		Note:       strings.TrimSpace(req.Note),
		TagIDs:     dedupeTags(req.TagIDs),
		Active:     active,
	}
	if err := h.db.Create(rb).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	c.JSON(201, rb)
}

// UpdateRecurringBill 更新固定账单模板。
func (h *Handler) UpdateRecurringBill(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "固定账单不存在")
		return
	}
	var rb model.RecurringBill
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&rb).Error; err != nil {
		notFound(c, "固定账单不存在")
		return
	}
	var req recurringReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if ok, msg := req.validate(); !ok {
		badRequest(c, msg)
		return
	}
	if !h.checkCategory(h.db, cu.ID, req.CategoryID, req.Type) {
		badRequest(c, "分类不存在或与账单类型不匹配")
		return
	}
	if req.AccountID != nil {
		var cnt int64
		h.db.Model(&model.Account{}).Where("id = ? AND user_id = ?", *req.AccountID, cu.ID).Count(&cnt)
		if cnt == 0 {
			badRequest(c, "账户不存在")
			return
		}
	}
	rb.Name = req.Name
	rb.Type = req.Type
	rb.CategoryID = req.CategoryID
	rb.AccountID = req.AccountID
	rb.Amount = model.Round2(req.Amount)
	rb.Day = req.Day
	rb.Note = strings.TrimSpace(req.Note)
	rb.TagIDs = dedupeTags(req.TagIDs)
	if req.Active != nil {
		rb.Active = *req.Active
	}
	if err := h.db.Save(&rb).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	c.JSON(200, rb)
}

// DeleteRecurringBill 删除固定账单模板。
func (h *Handler) DeleteRecurringBill(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "固定账单不存在")
		return
	}
	var rb model.RecurringBill
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&rb).Error; err != nil {
		notFound(c, "固定账单不存在")
		return
	}
	if err := h.db.Delete(&rb).Error; err != nil {
		fail(c, 500, "删除失败")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// ApplyRecurringBills 将勾选的固定账单批量记入指定月份（事务，任一项失败全部回滚）。
func (h *Handler) ApplyRecurringBills(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Month string `json:"month"`
		IDs   []uint `json:"ids"`
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
	if len(req.IDs) == 0 {
		badRequest(c, "请先勾选要记入的固定账单")
		return
	}
	var rbs []model.RecurringBill
	if err := h.db.Where("id IN ? AND user_id = ?", req.IDs, cu.ID).Find(&rbs).Error; err != nil {
		fail(c, 500, "查询失败")
		return
	}
	if len(rbs) == 0 {
		badRequest(c, "所选固定账单不存在")
		return
	}

	// 找出该月已由固定账单记入过的项（防止重复记账）
	start, end, _ := monthRange(req.Month)
	var doneIDs []uint
	h.db.Model(&model.Bill{}).
		Where("user_id = ? AND recurring_bill_id IN ? AND occurred_at >= ? AND occurred_at < ?",
			cu.ID, req.IDs, start, end).
		Distinct().Pluck("recurring_bill_id", &doneIDs)
	doneSet := map[uint]bool{}
	for _, id := range doneIDs {
		doneSet[id] = true
	}

	var toCreate []model.RecurringBill
	var duplicated []gin.H
	for _, rb := range rbs {
		if doneSet[rb.ID] {
			duplicated = append(duplicated, gin.H{"id": rb.ID, "name": rb.Name})
		} else {
			toCreate = append(toCreate, rb)
		}
	}

	if len(toCreate) == 0 {
		// 全部已在当月记过账
		c.JSON(200, gin.H{"count": 0, "items": []model.Bill{}, "duplicated": duplicated, "all_duplicated": true})
		return
	}

	created := make([]model.Bill, 0, len(toCreate))
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for _, rb := range toCreate {
			if !h.checkCategory(tx, cu.ID, rb.CategoryID, rb.Type) {
				return fmt.Errorf("「%s」的分类不存在或与账单类型不匹配", rb.Name)
			}
			day := rb.Day
			if day < 1 || day > 28 {
				day = 1
			}
			ocTime, err := parseDate(fmt.Sprintf("%s-%02d", req.Month, day))
			if err != nil {
				return fmt.Errorf("「%s」生成日期失败", rb.Name)
			}
			bill := &model.Bill{
				UserID:          cu.ID,
				CategoryID:      rb.CategoryID,
				AccountID:       rb.AccountID,
				Type:            rb.Type,
				Amount:          model.Round2(rb.Amount),
				Note:            rb.Note,
				OccurredAt:      model.DateTime{Time: ocTime},
				RecurringBillID: &rb.ID,
			}
			if err := tx.Create(bill).Error; err != nil {
				return err
			}
			if err := h.setBillTags(tx, bill.ID, cu.ID, rb.TagIDs); err != nil {
				return err
			}
			created = append(created, *bill)
		}
		return nil
	})
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	c.JSON(201, gin.H{"count": len(created), "items": created, "duplicated": duplicated})
}
