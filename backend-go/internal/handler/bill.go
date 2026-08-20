package handler

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

type billReq struct {
	CategoryID uint    `json:"category_id"`
	AccountID  *uint   `json:"account_id"`
	Type       string  `json:"type"`
	Amount     float64 `json:"amount"`
	Note       string  `json:"note"`
	OccurredAt string  `json:"occurred_at"`
}

// validate 校验并返回规范化后的账单。ok=false 表示校验失败。
func (r *billReq) validate() (bool, string) {
	if r.Type != model.TypeExpense && r.Type != model.TypeIncome {
		return false, "账单类型不正确"
	}
	if r.Amount <= 0 || r.Amount > 999999999 {
		return false, "金额需大于 0"
	}
	if utf8.RuneCountInString(r.Note) > 255 {
		return false, "备注不能超过 255 个字符"
	}
	return true, ""
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
func (h *Handler) checkCategory(cuID, catID uint, typ string) bool {
	var count int64
	h.db.Model(&model.Category{}).
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
	if err := q.Preload("Category").Preload("Account").
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
	if !h.checkCategory(cu.ID, req.CategoryID, req.Type) {
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
		UserID:     cu.ID,
		CategoryID: req.CategoryID,
		AccountID:  req.AccountID,
		Type:       req.Type,
		Amount:     model.Round2(req.Amount),
		Note:       strings.TrimSpace(req.Note),
		OccurredAt: model.DateTime{Time: req.occurredAt(true)},
	}
	if err := h.db.Create(bill).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	h.db.Preload("Category").Preload("Account").First(bill, bill.ID)
	c.JSON(201, bill)
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
	if !h.checkCategory(cu.ID, req.CategoryID, req.Type) {
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

	if err := h.db.Save(&bill).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	h.db.Preload("Category").Preload("Account").First(&bill, bill.ID)
	c.JSON(200, bill)
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
