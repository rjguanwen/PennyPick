package handler

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/middleware"
	"pennypickbackend/internal/model"
)

// GetOpLogSetting 查询操作日志开关状态（仅管理员）。
func (h *Handler) GetOpLogSetting(c *gin.Context) {
	enabled := middleware.OpLogEnabled(h.db)
	var count int64
	h.db.Model(&model.OperationLog{}).Count(&count)
	c.JSON(200, gin.H{"enabled": enabled, "count": count})
}

// SetOpLogSetting 开启/关闭操作日志（仅管理员）。
func (h *Handler) SetOpLogSetting(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	val := "false"
	if req.Enabled {
		val = "true"
	}
	var s model.SystemSetting
	if err := h.db.Where("key = ?", middleware.OpLogSettingKey).First(&s).Error; err == nil {
		s.Value = val
		s.UpdatedAt = time.Now()
		h.db.Save(&s)
	} else {
		h.db.Create(&model.SystemSetting{Key: middleware.OpLogSettingKey, Value: val, UpdatedAt: time.Now()})
	}
	c.JSON(200, gin.H{"enabled": req.Enabled})
}

// ListOpLogs 操作日志分页查询（仅管理员）。
func (h *Handler) ListOpLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	action := strings.TrimSpace(c.Query("action"))
	username := strings.TrimSpace(c.Query("username"))
	start := strings.TrimSpace(c.Query("start"))
	end := strings.TrimSpace(c.Query("end"))

	q := h.db.Model(&model.OperationLog{})
	if action != "" {
		q = q.Where("action LIKE ?", "%"+action+"%")
	}
	if username != "" {
		q = q.Where("username LIKE ?", "%"+username+"%")
	}
	if start != "" {
		q = q.Where("created_at >= ?", start)
	}
	if end != "" {
		q = q.Where("created_at < ?", end)
	}
	var total int64
	q.Count(&total)

	var logs []model.OperationLog
	q.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	list := make([]gin.H, 0, len(logs))
	for _, l := range logs {
		list = append(list, gin.H{
			"id":         l.ID,
			"username":   l.Username,
			"action":     l.Action,
			"method":     l.Method,
			"path":       l.Path,
			"ip":         l.IP,
			"status":     l.Status,
			"created_at": l.CreatedAt,
		})
	}
	c.JSON(200, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}
