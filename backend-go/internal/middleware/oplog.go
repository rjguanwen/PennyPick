package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pennypickbackend/internal/model"
)

// OpLogSettingKey 操作日志开关的配置键。
const OpLogSettingKey = "op_log_enabled"

// OpLogEnabled 读取操作日志开关（默认关闭）。
func OpLogEnabled(db *gorm.DB) bool {
	var v string
	db.Model(&model.SystemSetting{}).Where("key = ?", OpLogSettingKey).Pluck("value", &v)
	return v == "true"
}

// statusWriter 捕获响应状态码，供中间件判断请求是否成功。
type statusWriter struct {
	gin.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// actionName 根据方法与路径生成中文操作描述。
func actionName(method, path string) string {
	p := strings.TrimSuffix(path, "/")
	switch method {
	case http.MethodPost:
		switch {
		case p == "/api/auth/login":
			return "登录"
		case p == "/api/auth/register":
			return "注册"
		case strings.HasPrefix(p, "/api/bills/batch"):
			return "批量记账"
		case strings.HasPrefix(p, "/api/bills"):
			return "记一笔"
		case strings.HasPrefix(p, "/api/bill-import"):
			return "导入账单"
		case strings.HasPrefix(p, "/api/categories"):
			return "新建分类"
		case strings.HasPrefix(p, "/api/accounts"):
			return "新建账户"
		case strings.HasPrefix(p, "/api/budgets"):
			return "保存预算"
		case strings.HasPrefix(p, "/api/recurring-bills"):
			return "固定账单"
		case strings.HasPrefix(p, "/api/repayments"):
			return "还款管理"
		case strings.HasPrefix(p, "/api/tags"):
			return "新建标签"
		case strings.HasPrefix(p, "/api/reports"):
			return "生成报告"
		}
	case http.MethodPut, http.MethodPatch:
		switch {
		case strings.HasPrefix(p, "/api/bills"):
			return "修改账单"
		case strings.HasPrefix(p, "/api/categories"):
			return "修改分类"
		case strings.HasPrefix(p, "/api/accounts"):
			return "修改账户"
		case strings.HasPrefix(p, "/api/budgets"):
			return "保存预算"
		case strings.HasPrefix(p, "/api/recurring-bills"):
			return "固定账单"
		case strings.HasPrefix(p, "/api/repayments"):
			return "还款管理"
		case strings.HasPrefix(p, "/api/tags"):
			return "修改标签"
		case strings.HasPrefix(p, "/api/settings"):
			return "系统设置"
		}
	case http.MethodDelete:
		switch {
		case strings.HasPrefix(p, "/api/bills"):
			return "删除账单"
		case strings.HasPrefix(p, "/api/categories"):
			return "删除分类"
		case strings.HasPrefix(p, "/api/accounts"):
			return "删除账户"
		case strings.HasPrefix(p, "/api/budgets"):
			return "删除预算"
		case strings.HasPrefix(p, "/api/recurring-bills"):
			return "固定账单"
		case strings.HasPrefix(p, "/api/repayments"):
			return "还款管理"
		case strings.HasPrefix(p, "/api/tags"):
			return "删除标签"
		}
	}
	return method + " " + path
}

// OpLogger 操作日志中间件：开关开启时记录非 GET/OPTIONS/HEAD 的成功写操作。
func OpLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		m := c.Request.Method
		if m == http.MethodGet || m == http.MethodOptions || m == http.MethodHead {
			c.Next()
			return
		}
		if !OpLogEnabled(db) {
			c.Next()
			return
		}
		w := &statusWriter{ResponseWriter: c.Writer, status: http.StatusOK}
		c.Writer = w
		c.Next()
		if w.status >= http.StatusBadRequest {
			return // 请求失败不记录
		}
		// 用户信息：优先取认证上下文；登录请求从表单取用户名
		var userID uint
		var username string
		if u, ok := c.Get("user"); ok {
			if cu, ok2 := u.(*UserContext); ok2 {
				userID = cu.ID
				username = cu.Username
			}
		} else if m == http.MethodPost && strings.HasSuffix(c.Request.URL.Path, "/auth/login") {
			username = strings.TrimSpace(c.PostForm("username"))
		}
		entry := &model.OperationLog{
			UserID:    userID,
			Username:  username,
			Action:    actionName(m, c.Request.URL.Path),
			Method:    m,
			Path:      c.Request.URL.Path,
			IP:        c.ClientIP(),
			Status:    w.status,
			CreatedAt: time.Now(),
		}
		_ = db.Create(entry).Error
	}
}
