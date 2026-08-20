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
		Name string `json:"name"`
		Icon string `json:"icon"`
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
	var maxSort int
	h.db.Model(&model.Account{}).Where("user_id = ?", cu.ID).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort)
	acc := &model.Account{
		UserID:    cu.ID,
		Name:      req.Name,
		Icon:      strings.TrimSpace(req.Icon),
		SortOrder: maxSort + 1,
	}
	if err := h.db.Create(acc).Error; err != nil {
		fail(c, 500, "创建账户失败")
		return
	}
	c.JSON(201, acc)
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
