package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
	"pennypickbackend/internal/service"
)

const maxImportFileSize = 10 << 20 // 10MB

// ParseBillImport 上传账单文件并解析（预览阶段：去重/过滤标记，不落库）。
func (h *Handler) ParseBillImport(c *gin.Context) {
	cu := currentUser(c)
	platform := strings.TrimSpace(c.PostForm("platform"))
	fileHeader, err := c.FormFile("file")
	if err != nil {
		badRequest(c, "请选择要上传的账单文件")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		badRequest(c, "读取文件失败")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxImportFileSize))
	if err != nil {
		badRequest(c, "读取文件失败")
		return
	}
	if len(data) == 0 {
		badRequest(c, "文件内容为空")
		return
	}
	if len(data) >= maxImportFileSize {
		badRequest(c, "文件过大，请上传 10MB 以内的账单文件")
		return
	}
	svc := service.NewBillImportService(h.db)
	res, err := svc.Parse(service.ParseRequest{
		Platform: platform,
		FileName: fileHeader.Filename,
		Data:     data,
	}, cu.ID)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	c.JSON(200, res)
}

// ConfirmBillImport 确认导入：事务写入账单与导入明细。
func (h *Handler) ConfirmBillImport(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		Platform  string             `json:"platform"`
		FileName  string             `json:"file_name"`
		AccountID uint               `json:"account_id"`
		Items     []model.ImportItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if req.Platform != model.PlatformWechat && req.Platform != model.PlatformAlipay {
		badRequest(c, "平台参数不正确")
		return
	}
	if len(req.Items) == 0 {
		badRequest(c, "没有可导入的账单记录")
		return
	}
	svc := service.NewBillImportService(h.db)
	res, err := svc.Confirm(service.ConfirmRequest{
		UserID:    cu.ID,
		Platform:  req.Platform,
		FileName:  req.FileName,
		AccountID: req.AccountID,
		Items:     req.Items,
	})
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	c.JSON(200, res)
}

// ListImportHistory 导入历史（分页）。
func (h *Handler) ListImportHistory(c *gin.Context) {
	cu := currentUser(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 50 {
		pageSize = 50
	}
	var total int64
	h.db.Model(&model.BillImport{}).Where("user_id = ?", cu.ID).Count(&total)
	var list []model.BillImport
	if err := h.db.Where("user_id = ?", cu.ID).
		Order("id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&list).Error; err != nil {
		fail(c, 500, "查询导入历史失败")
		return
	}
	c.JSON(200, gin.H{"items": list, "total": total, "page": page, "page_size": pageSize})
}

// GetImportDetail 某次导入任务的明细。
func (h *Handler) GetImportDetail(c *gin.Context) {
	cu := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		badRequest(c, "导入记录不存在")
		return
	}
	var imp model.BillImport
	if err := h.db.Where("id = ? AND user_id = ?", id, cu.ID).First(&imp).Error; err != nil {
		notFound(c, "导入记录不存在")
		return
	}
	var items []model.BillImportItem
	if err := h.db.Where("import_id = ?", id).Order("id asc").Find(&items).Error; err != nil {
		fail(c, 500, "查询导入明细失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"import": imp, "items": items})
}
