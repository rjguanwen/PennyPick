package handler

import (
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"pennypickbackend/internal/model"
)

// Login 登录（form-data: username/password）。
func (h *Handler) Login(c *gin.Context) {
	username := strings.TrimSpace(c.PostForm("username"))
	password := c.PostForm("password")
	if username == "" || password == "" {
		badRequest(c, "请输入用户名和密码")
		return
	}
	var u model.User
	if err := h.db.Where("username = ?", username).First(&u).Error; err != nil {
		fail(c, 401, "用户名或密码错误")
		return
	}
	if !u.CheckPassword(password) {
		fail(c, 401, "用户名或密码错误")
		return
	}
	if !u.IsActive {
		fail(c, 403, "账号已被停用")
		return
	}
	token, err := h.auth.CreateToken(u.ID)
	if err != nil {
		fail(c, 500, "生成凭证失败")
		return
	}
	c.JSON(200, gin.H{"access_token": token, "user": u})
}

// Register 注册新用户（JSON），自动初始化预置分类与账户。
func (h *Handler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Nickname = strings.TrimSpace(req.Nickname)
	if n := utf8.RuneCountInString(req.Username); n < 3 || n > 32 {
		badRequest(c, "用户名长度需为 3-32 个字符")
		return
	}
	if n := utf8.RuneCountInString(req.Password); n < 6 || n > 64 {
		badRequest(c, "密码长度需为 6-64 个字符")
		return
	}
	var count int64
	h.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		badRequest(c, "用户名已存在")
		return
	}
	if req.Nickname == "" {
		req.Nickname = req.Username
	}
	u := &model.User{
		Username:       req.Username,
		HashedPassword: model.HashPassword(req.Password),
		Nickname:       req.Nickname,
		IsActive:       true,
	}
	if err := h.db.Create(u).Error; err != nil {
		fail(c, 500, "注册失败，请稍后重试")
		return
	}
	model.SeedDefaultCategories(h.db, u.ID)
	model.SeedDefaultAccounts(h.db, u.ID)
	c.JSON(201, gin.H{"id": u.ID, "username": u.Username, "nickname": u.Nickname})
}

// Me 当前登录用户信息。
func (h *Handler) Me(c *gin.Context) {
	cu := currentUser(c)
	var u model.User
	if err := h.db.First(&u, cu.ID).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}
	c.JSON(200, u)
}

// ChangePassword 修改密码（JSON: old_password / new_password）。
func (h *Handler) ChangePassword(c *gin.Context) {
	cu := currentUser(c)
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "请求参数有误")
		return
	}
	if n := utf8.RuneCountInString(req.NewPassword); n < 6 || n > 64 {
		badRequest(c, "新密码长度需为 6-64 个字符")
		return
	}
	var u model.User
	if err := h.db.First(&u, cu.ID).Error; err != nil {
		notFound(c, "用户不存在")
		return
	}
	if !u.CheckPassword(req.OldPassword) {
		badRequest(c, "原密码不正确")
		return
	}
	u.SetPassword(req.NewPassword)
	if err := h.db.Model(&u).Update("hashed_password", u.HashedPassword).Error; err != nil {
		fail(c, 500, "修改失败，请稍后重试")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
