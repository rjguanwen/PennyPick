package handler

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// ListTags 标签列表（按名称排序）。
func (h *Handler) ListTags(c *gin.Context) {
	cu := currentUser(c)
	var tags []model.Tag
	if err := h.db.Where("user_id = ?", cu.ID).Order("name asc, id asc").Find(&tags).Error; err != nil {
		fail(c, 500, "查询标签失败")
		return
	}
	c.JSON(200, tags)
}

// CreateTag 新建标签（幂等：同名已存在时直接返回已有标签）。
func (h *Handler) CreateTag(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(req.Name); n < 1 || n > 16 {
		badRequest(c, "标签名称需为 1-16 个字符")
		return
	}
	var existing model.Tag
	if err := h.db.Where("user_id = ? AND name = ?", cu.ID, req.Name).First(&existing).Error; err == nil {
		c.JSON(200, existing)
		return
	}
	tag := &model.Tag{UserID: cu.ID, Name: req.Name}
	if err := h.db.Create(tag).Error; err != nil {
		fail(c, 500, "创建标签失败")
		return
	}
	c.JSON(201, tag)
}

// UpdateTag 修改标签名称。
func (h *Handler) UpdateTag(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "标签不存在")
		return
	}
	var tag model.Tag
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&tag).Error; err != nil {
		notFound(c, "标签不存在")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(req.Name); n < 1 || n > 16 {
		badRequest(c, "标签名称需为 1-16 个字符")
		return
	}
	var dup int64
	h.db.Model(&model.Tag{}).Where("user_id = ? AND name = ? AND id <> ?", cu.ID, req.Name, tag.ID).Count(&dup)
	if dup > 0 {
		badRequest(c, "已存在同名标签")
		return
	}
	tag.Name = req.Name
	if err := h.db.Save(&tag).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	c.JSON(200, tag)
}

// DeleteTag 删除标签（同时清理账单关联）。
func (h *Handler) DeleteTag(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "标签不存在")
		return
	}
	var tag model.Tag
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&tag).Error; err != nil {
		notFound(c, "标签不存在")
		return
	}
	if err := h.db.Exec("DELETE FROM bill_tags WHERE tag_id = ?", tag.ID).Error; err != nil {
		fail(c, 500, "删除失败")
		return
	}
	if err := h.db.Delete(&tag).Error; err != nil {
		fail(c, 500, "删除失败")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
