package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pennypickbackend/internal/model"
)

type billReq struct {
	CategoryID   uint    `json:"category_id"`
	AccountID    *uint   `json:"account_id"`
	Type         string  `json:"type"`
	Amount       float64 `json:"amount"`
	Note         string  `json:"note"`
	OccurredAt   string  `json:"occurred_at"`
	RefundAmount float64 `json:"refund_amount"` // 退款金额（仅支出可登记，0 表示未退款）
	RefundedAt   string  `json:"refunded_at"`   // 退款时间（YYYY-MM-DD 或 YYYY-MM-DD HH:mm，可选）
	TagIDs       []uint  `json:"tag_ids"`
}

// validate 校验并返回规范化后的账单。ok=false 表示校验失败。
func (r *billReq) validate() (bool, string) {
	if r.Type != model.TypeExpense && r.Type != model.TypeIncome {
		return false, "账单类型不正确"
	}
	if r.Amount <= 0 || r.Amount > 999999999 {
		return false, "金额需大于 0"
	}
	if r.RefundAmount < 0 || r.RefundAmount > r.Amount {
		return false, "退款金额需在 0 与账单金额之间"
	}
	if r.Type != model.TypeExpense && r.RefundAmount > 0 {
		return false, "仅支出账单可登记退款"
	}
	if utf8.RuneCountInString(r.Note) > 255 {
		return false, "备注不能超过 255 个字符"
	}
	return true, ""
}

// refundedAt 解析退款时间；未填写且登记退款时返回当前时间，未退款返回 nil。
func (r *billReq) refundedAt() *model.DateTime {
	if r.RefundAmount <= 0 {
		return nil
	}
	if s := strings.TrimSpace(r.RefundedAt); s != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
			return &model.DateTime{Time: t}
		}
		if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
			return &model.DateTime{Time: t}
		}
	}
	return &model.DateTime{Time: time.Now()}
}

func (r *billReq) occurredAt(defaultNow bool) time.Time {
	if s := strings.TrimSpace(r.OccurredAt); s != "" {
		if t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local); err == nil {
			return t
		}
		if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
			return t
		}
	}
	if defaultNow {
		return time.Now()
	}
	return time.Time{}
}

// checkCategory 校验分类归属与类型匹配。
func (h *Handler) checkCategory(db *gorm.DB, cuID, catID uint, typ string) bool {
	var count int64
	db.Model(&model.Category{}).
		Where("id = ? AND user_id = ? AND type = ?", catID, cuID, typ).Count(&count)
	return count > 0
}

// ListBills 账单列表，支持 month / start+end / type / category_id / account_id / keyword 过滤与分页。
func (h *Handler) ListBills(c *gin.Context) {
	cu := currentUser(c)

	q := h.db.Model(&model.Bill{}).Where("user_id = ?", cu.ID)

	if month := strings.TrimSpace(c.Query("month")); month != "" {
		start, end, ok := monthRange(month)
		if !ok {
			badRequest(c, "月份格式不正确")
			return
		}
		q = q.Where("occurred_at >= ? AND occurred_at < ?", start, end)
	} else {
		if s := strings.TrimSpace(c.Query("start")); s != "" {
			if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
				q = q.Where("occurred_at >= ?", t)
			}
		}
		if e := strings.TrimSpace(c.Query("end")); e != "" {
			if t, err := time.ParseInLocation("2006-01-02", e, time.Local); err == nil {
				q = q.Where("occurred_at < ?", t.AddDate(0, 0, 1))
			}
		}
	}
	if typ := c.Query("type"); typ == model.TypeExpense || typ == model.TypeIncome {
		q = q.Where("type = ?", typ)
	}
	if catID := c.Query("category_id"); catID != "" {
		q = q.Where("category_id = ?", catID)
	}
	if accID := c.Query("account_id"); accID != "" {
		q = q.Where("account_id = ?", accID)
	}
	if tagID := c.Query("tag_id"); tagID != "" {
		q = q.Where("id IN (SELECT bill_id FROM bill_tags WHERE tag_id = ?)", tagID)
	}
	if rf := c.Query("refunded"); rf == "true" {
		q = q.Where("refund_amount > 0")
	} else if rf == "false" {
		q = q.Where("refund_amount = 0")
	}
	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		q = q.Where("note LIKE ?", "%"+kw+"%")
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > 200 {
		pageSize = 200
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		fail(c, 500, "查询账单失败")
		return
	}
	var bills []model.Bill
	if err := q.Preload("Category").Preload("Account").Preload("Tags").
		Order("occurred_at desc, id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&bills).Error; err != nil {
		fail(c, 500, "查询账单失败")
		return
	}
	c.JSON(200, gin.H{"items": bills, "total": total, "page": page, "page_size": pageSize})
}

// CreateBill 记一笔。
func (h *Handler) CreateBill(c *gin.Context) {
	cu := currentUser(c)
	var req billReq
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

	bill := &model.Bill{
		UserID:       cu.ID,
		CategoryID:   req.CategoryID,
		AccountID:    req.AccountID,
		Type:         req.Type,
		Amount:       model.Round2(req.Amount),
		Note:         strings.TrimSpace(req.Note),
		OccurredAt:   model.DateTime{Time: req.occurredAt(true)},
		RefundAmount: model.Round2(req.RefundAmount),
		RefundedAt:   req.refundedAt(),
	}
	if err := h.db.Create(bill).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	if err := h.setBillTags(h.db, bill.ID, cu.ID, req.TagIDs); err != nil {
		h.db.Delete(bill)
		fail(c, 400, err.Error())
		return
	}
	h.db.Preload("Category").Preload("Account").Preload("Tags").First(bill, bill.ID)
	// 检查是否落入已标记还款的账期，是则附带提醒（不阻止保存）
	var accID uint
	if bill.AccountID != nil {
		accID = *bill.AccountID
	}
	c.JSON(201, struct {
		model.Bill
		RepaymentWarning string `json:"repayment_warning"`
	}{Bill: *bill, RepaymentWarning: repaymentWarnText(h.repaymentMarkedFor(cu.ID, accID, bill.OccurredAt.Time, bill.Type))})
}

// UpdateBill 修改账单（整体更新）。
func (h *Handler) UpdateBill(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "账单不存在")
		return
	}
	var bill model.Bill
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&bill).Error; err != nil {
		notFound(c, "账单不存在")
		return
	}
	var req billReq
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
	if req.OccurredAt != "" {
		bill.OccurredAt = model.DateTime{Time: req.occurredAt(false)}
	}
	bill.CategoryID = req.CategoryID
	bill.AccountID = req.AccountID
	bill.Type = req.Type
	bill.Amount = model.Round2(req.Amount)
	bill.Note = strings.TrimSpace(req.Note)
	bill.RefundAmount = model.Round2(req.RefundAmount)
	bill.RefundedAt = req.refundedAt()

	if err := h.db.Save(&bill).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	if err := h.setBillTags(h.db, bill.ID, cu.ID, req.TagIDs); err != nil {
		fail(c, 400, err.Error())
		return
	}
	h.db.Preload("Category").Preload("Account").Preload("Tags").First(&bill, bill.ID)
	// 检查修改后的账单是否落入已标记还款的账期，是则附带提醒（不阻止保存）
	var accID uint
	if bill.AccountID != nil {
		accID = *bill.AccountID
	}
	c.JSON(200, struct {
		model.Bill
		RepaymentWarning string `json:"repayment_warning"`
	}{Bill: bill, RepaymentWarning: repaymentWarnText(h.repaymentMarkedFor(cu.ID, accID, bill.OccurredAt.Time, bill.Type))})
}

// DeleteBill 删除账单。
func (h *Handler) DeleteBill(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "账单不存在")
		return
	}
	var bill model.Bill
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&bill).Error; err != nil {
		notFound(c, "账单不存在")
		return
	}
	if err := h.db.Delete(&bill).Error; err != nil {
		fail(c, 500, "删除失败")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// setBillTags 校验并替换账单标签（上限、归属校验）。
func (h *Handler) setBillTags(db *gorm.DB, billID, userID uint, tagIDs []uint) error {
	tagIDs = dedupeTags(tagIDs)
	if len(tagIDs) > model.MaxBillTags {
		return fmt.Errorf("每笔账单最多添加 %d 个标签", model.MaxBillTags)
	}
	if err := db.Exec("DELETE FROM bill_tags WHERE bill_id = ?", billID).Error; err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	var tags []model.Tag
	if err := db.Where("id IN ? AND user_id = ?", tagIDs, userID).Find(&tags).Error; err != nil {
		return err
	}
	if len(tags) != len(tagIDs) {
		return fmt.Errorf("部分标签不存在或无权使用")
	}
	for _, t := range tags {
		if err := db.Exec("INSERT INTO bill_tags (bill_id, tag_id) VALUES (?, ?)", billID, t.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

// BatchCreateBills 批量记多笔账单（同一账户，事务整体提交，任一笔失败则全部回滚）。
func (h *Handler) BatchCreateBills(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		AccountID uint      `json:"account_id"`
		Items     []billReq `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if len(req.Items) == 0 {
		badRequest(c, "请至少录入一笔账单")
		return
	}
	if len(req.Items) > 200 {
		badRequest(c, "单次最多录入 200 笔")
		return
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ?", req.AccountID, cu.ID).First(&acc).Error; err != nil {
		badRequest(c, "账户不存在")
		return
	}
	created := make([]model.Bill, 0, len(req.Items))
	err := h.db.Transaction(func(tx *gorm.DB) error {
		for i := range req.Items {
			it := &req.Items[i]
			if it.AccountID != nil && *it.AccountID != req.AccountID {
				return fmt.Errorf("第 %d 笔：账户与所选账户不一致", i+1)
			}
			if ok, msg := it.validate(); !ok {
				return errors.New("第 " + strconv.Itoa(i+1) + " 笔：" + msg)
			}
			if !h.checkCategory(tx, cu.ID, it.CategoryID, it.Type) {
				return fmt.Errorf("第 %d 笔：分类不存在或与账单类型不匹配", i+1)
			}
			bill := &model.Bill{
				UserID:       cu.ID,
				CategoryID:   it.CategoryID,
				AccountID:    &req.AccountID,
				Type:         it.Type,
				Amount:       model.Round2(it.Amount),
				Note:         strings.TrimSpace(it.Note),
				OccurredAt:   model.DateTime{Time: it.occurredAt(true)},
				RefundAmount: model.Round2(it.RefundAmount),
				RefundedAt:   it.refundedAt(),
			}
			if err := tx.Create(bill).Error; err != nil {
				return err
			}
			if err := h.setBillTags(tx, bill.ID, cu.ID, it.TagIDs); err != nil {
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
	// 检查是否有账单落入已标记还款的账期，是则附带提醒（不阻止保存）
	var hits []*repaymentHit
	for i := range created {
		b := &created[i]
		var accID uint
		if b.AccountID != nil {
			accID = *b.AccountID
		}
		if hit := h.repaymentMarkedFor(cu.ID, accID, b.OccurredAt.Time, b.Type); hit != nil {
			hits = append(hits, hit)
		}
	}
	c.JSON(201, gin.H{"count": len(created), "items": created, "repayment_warning": batchRepaymentWarnText(hits)})
}

// dedupeTags 去重并保持顺序。
func dedupeTags(ids []uint) []uint {
	seen := map[uint]bool{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}
