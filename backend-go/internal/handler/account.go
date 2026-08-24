package handler

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// ListAccounts 账户列表。
func (h *Handler) ListAccounts(c *gin.Context) {
	cu := currentUser(c)
	var accounts []model.Account
	if err := h.db.Where("user_id = ?", cu.ID).Order("sort_order asc, id asc").Find(&accounts).Error; err != nil {
		fail(c, 500, "查询账户失败")
		return
	}
	c.JSON(200, accounts)
}

// CreateAccount 新建账户。
func (h *Handler) CreateAccount(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Name         string `json:"name"`
		Icon         string `json:"icon"`
		IsCredit     bool   `json:"is_credit"`
		StatementDay int    `json:"statement_day"`
		RepayDay     int    `json:"repay_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 32 {
		badRequest(c, "账户名称需为 1-32 个字符")
		return
	}
	if req.IsCredit {
		if req.StatementDay < 1 || req.StatementDay > 28 {
			req.StatementDay = 1
		}
		if req.RepayDay < 1 || req.RepayDay > 28 {
			req.RepayDay = 25
		}
	} else {
		req.StatementDay = 0
		req.RepayDay = 0
	}
	var maxSort int
	h.db.Model(&model.Account{}).Where("user_id = ?", cu.ID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort)
	acc := &model.Account{
		UserID:       cu.ID,
		Name:         req.Name,
		Icon:         strings.TrimSpace(req.Icon),
		IsCredit:     req.IsCredit,
		StatementDay: req.StatementDay,
		RepayDay:     req.RepayDay,
		SortOrder:    maxSort + 1,
	}
	if err := h.db.Create(acc).Error; err != nil {
		fail(c, 500, "创建账户失败")
		return
	}
	c.JSON(201, acc)
}

// UpdateAccount 更新账户（名称/图标/是否信用账户/还款日）。
func (h *Handler) UpdateAccount(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "账户不存在")
		return
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&acc).Error; err != nil {
		notFound(c, "账户不存在")
		return
	}
	var req struct {
		Name         string `json:"name"`
		Icon         string `json:"icon"`
		IsCredit     bool   `json:"is_credit"`
		StatementDay int    `json:"statement_day"`
		RepayDay     int    `json:"repay_day"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || utf8.RuneCountInString(req.Name) > 32 {
		badRequest(c, "账户名称需为 1-32 个字符")
		return
	}
	acc.Name = req.Name
	acc.Icon = strings.TrimSpace(req.Icon)
	acc.IsCredit = req.IsCredit
	if req.IsCredit {
		if req.StatementDay < 1 || req.StatementDay > 28 {
			req.StatementDay = 1
		}
		if req.RepayDay < 1 || req.RepayDay > 28 {
			req.RepayDay = 25
		}
		acc.StatementDay = req.StatementDay
		acc.RepayDay = req.RepayDay
	} else {
		acc.StatementDay = 0
		acc.RepayDay = 0
	}
	if err := h.db.Save(&acc).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	c.JSON(200, acc)
}

// DeleteAccount 删除账户（已有账单时禁止）。
func (h *Handler) DeleteAccount(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "账户不存在")
		return
	}
	var acc model.Account
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&acc).Error; err != nil {
		notFound(c, "账户不存在")
		return
	}
	var cnt int64
	h.db.Model(&model.Bill{}).Where("user_id = ? AND account_id = ?", cu.ID, id).Count(&cnt)
	if cnt > 0 {
		forbidden(c, "该账户下已有账单，无法删除")
		return
	}
	if err := h.db.Delete(&acc).Error; err != nil {
		fail(c, 500, "删除失败")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
