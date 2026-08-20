package handler

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// fixedCategoryName 内置固定分类名：始终排在末尾，且不允许修改/删除。
const fixedCategoryName = "其他"

type categoryReq struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

// reorderFixedLast 按类型分组重排：保证每个类型组内「其他」位于末尾。
func reorderFixedLast(cats []model.Category) []model.Category {
	if len(cats) < 2 {
		return cats
	}
	typeGroups := map[string][]model.Category{}
	groupOrder := []string{}
	for _, c := range cats {
		if _, ok := typeGroups[c.Type]; !ok {
			groupOrder = append(groupOrder, c.Type)
		}
		typeGroups[c.Type] = append(typeGroups[c.Type], c)
	}
	out := make([]model.Category, 0, len(cats))
	for _, t := range groupOrder {
		group := typeGroups[t]
		for i, c := range group {
			if c.Name == fixedCategoryName {
				// 移动到该组末尾
				group = append(append(group[:i:i], group[i+1:]...), c)
				break
			}
		}
		out = append(out, group...)
	}
	return out
}

func (r *categoryReq) valid() string {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || utf8.RuneCountInString(r.Name) > 32 {
		return "分类名称需为 1-32 个字符"
	}
	if r.Type != model.TypeExpense && r.Type != model.TypeIncome {
		return "分类类型不正确"
	}
	return ""
}

// ListCategories 分类列表，可带 type 过滤，并计算近30天使用次数用于排序。
func (h *Handler) ListCategories(c *gin.Context) {
	cu := currentUser(c)
	typ := c.Query("type")

	q := h.db.Where("user_id = ?", cu.ID)
	if typ != "" {
		q = q.Where("type = ?", typ)
	}
	var cats []model.Category
	if err := q.Order("sort_order asc, id asc").Find(&cats).Error; err != nil {
		fail(c, 500, "查询分类失败")
		return
	}
	cats = reorderFixedLast(cats)

	// 近30天各分类使用次数
	recent := map[uint]int{}
	since := time.Now().AddDate(0, 0, -30)
	var rows []struct {
		CategoryID uint
		Cnt        int
	}
	h.db.Model(&model.Bill{}).
		Select("category_id, count(*) as cnt").
		Where("user_id = ? AND occurred_at >= ?", cu.ID, since).
		Group("category_id").Scan(&rows)
	for _, r := range rows {
		recent[r.CategoryID] = r.Cnt
	}
	for i := range cats {
		cats[i].RecentCount = recent[cats[i].ID]
	}
	c.JSON(200, cats)
}

// CreateCategory 新建分类。
func (h *Handler) CreateCategory(c *gin.Context) {
	cu := currentUser(c)
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if msg := req.valid(); msg != "" {
		badRequest(c, msg)
		return
	}
	if req.Name == fixedCategoryName {
		badRequest(c, "「其他」为内置分类，请使用其他名称")
		return
	}
	var maxSort int
	h.db.Model(&model.Category{}).Where("user_id = ? AND type = ?", cu.ID, req.Type).
		Select("COALESCE(MAX(sort_order), 0)").Scan(&maxSort)

	cat := &model.Category{
		UserID:    cu.ID,
		Name:      req.Name,
		Type:      req.Type,
		Icon:      strings.TrimSpace(req.Icon),
		Color:     strings.TrimSpace(req.Color),
		SortOrder: maxSort + 1,
	}
	if err := h.db.Create(cat).Error; err != nil {
		fail(c, 500, "创建分类失败")
		return
	}
	c.JSON(201, cat)
}

// UpdateCategory 更新分类。
func (h *Handler) UpdateCategory(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "分类不存在")
		return
	}
	var cat model.Category
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&cat).Error; err != nil {
		notFound(c, "分类不存在")
		return
	}
	if cat.Name == fixedCategoryName {
		forbidden(c, "「其他」为内置分类，不支持修改")
		return
	}
	var req categoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if msg := req.valid(); msg != "" {
		badRequest(c, msg)
		return
	}
	cat.Name = req.Name
	cat.Icon = strings.TrimSpace(req.Icon)
	cat.Color = strings.TrimSpace(req.Color)
	if err := h.db.Save(&cat).Error; err != nil {
		fail(c, 500, "保存失败")
		return
	}
	c.JSON(200, cat)
}

// DeleteCategory 删除分类（已有账单时禁止）。
func (h *Handler) DeleteCategory(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "分类不存在")
		return
	}
	var cat model.Category
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&cat).Error; err != nil {
		notFound(c, "分类不存在")
		return
	}
	if cat.Name == fixedCategoryName {
		forbidden(c, "「其他」为内置分类，不支持删除")
		return
	}
	var cnt int64
	h.db.Model(&model.Bill{}).Where("user_id = ? AND category_id = ?", cu.ID, id).Count(&cnt)
	if cnt > 0 {
		forbidden(c, "该分类下已有账单，无法删除")
		return
	}
	if err := h.db.Delete(&cat).Error; err != nil {
		fail(c, 500, "删除失败")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
