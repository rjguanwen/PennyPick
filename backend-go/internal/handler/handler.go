package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"pennypickbackend/internal/config"
	"pennypickbackend/internal/middleware"
)

// Handler 持有依赖：数据库、配置、认证
type Handler struct {
	db   *gorm.DB
	cfg  *config.Config
	auth *middleware.Auth
}

func New(db *gorm.DB, cfg *config.Config, auth *middleware.Auth) *Handler {
	return &Handler{db: db, cfg: cfg, auth: auth}
}

// currentUser 从上下文获取当前登录用户
func currentUser(c *gin.Context) *middleware.UserContext {
	v, ok := c.Get("user")
	if !ok {
		return nil
	}
	return v.(*middleware.UserContext)
}

// RegisterRoutes 注册全部路由
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api", middleware.OpLogger(h.db))

	// 公开接口
	api.POST("/auth/login", h.Login)
	api.POST("/auth/register", h.Register)

	// 需要登录
	user := api.Group("", h.auth.RequireUser())
	user.GET("/auth/me", h.Me)
	user.PUT("/auth/password", h.ChangePassword)

	// 分类
	user.GET("/categories", h.ListCategories)
	user.POST("/categories", h.CreateCategory)
	user.PATCH("/categories/:id", h.UpdateCategory)
	user.DELETE("/categories/:id", h.DeleteCategory)

	// 账户
	user.GET("/accounts", h.ListAccounts)
	user.POST("/accounts", h.CreateAccount)
	user.PATCH("/accounts/:id", h.UpdateAccount)
	user.DELETE("/accounts/:id", h.DeleteAccount)

	// 还款
	user.GET("/repayments", h.ListRepayments)
	user.GET("/repayments/bills", h.ListRepaymentBills)
	user.POST("/repayments", h.MarkRepayment)
	user.DELETE("/repayments", h.UnmarkRepayment)

	// 固定账单
	user.GET("/recurring-bills", h.ListRecurringBills)
	user.POST("/recurring-bills", h.CreateRecurringBill)
	user.PATCH("/recurring-bills/:id", h.UpdateRecurringBill)
	user.DELETE("/recurring-bills/:id", h.DeleteRecurringBill)
	user.POST("/recurring-bills/apply", h.ApplyRecurringBills)

	// 收支报告（月度 + 年度）
	user.GET("/reports", h.ListReports)
	user.POST("/reports/generate", h.GenerateReport)
	user.POST("/reports/yearly", h.GenerateYearlyReport)
	user.GET("/reports/yearly/list", h.ListYearlyReports)
	user.GET("/reports/yearly/:id", h.GetYearlyReport)
	user.GET("/reports/:id", h.GetReport)

	// 标签
	user.GET("/tags", h.ListTags)
	user.POST("/tags", h.CreateTag)
	user.PATCH("/tags/:id", h.UpdateTag)
	user.DELETE("/tags/:id", h.DeleteTag)

	// 账单
	user.GET("/bills", h.ListBills)
	user.POST("/bills", h.CreateBill)
	user.POST("/bills/batch", h.BatchCreateBills)
	user.PATCH("/bills/:id", h.UpdateBill)
	user.DELETE("/bills/:id", h.DeleteBill)

	// 账单导入
	user.POST("/bill-import/parse", h.ParseBillImport)
	user.POST("/bill-import/confirm", h.ConfirmBillImport)
	user.GET("/bill-import/history", h.ListImportHistory)
	user.GET("/bill-import/:id", h.GetImportDetail)

	// 预算（总预算）
	user.GET("/budgets", h.GetBudget)
	user.GET("/budgets/all", h.ListBudgets)
	user.PUT("/budgets", h.UpsertBudget)
	user.DELETE("/budgets", h.DeleteBudget)

	// 预算（分类预算）
	user.GET("/budgets/categories", h.ListCategoryBudgets)
	user.PUT("/budgets/category", h.UpsertCategoryBudget)
	user.DELETE("/budgets/category", h.DeleteCategoryBudget)
	// 预算复制
	user.POST("/budgets/copy", h.CopyBudget)

	// 统计
	user.GET("/stats/overview", h.Overview)
	user.GET("/stats/by-category", h.ByCategory)
	user.GET("/stats/trend", h.Trend)
	user.GET("/stats/accounts", h.AccountStats)
	user.GET("/stats/tags", h.Tags)

	// 导出
	user.GET("/export", h.ExportBills)

	// 操作日志（仅管理员：查看 + 开关设置）
	admin := user.Group("", h.auth.RequireAdmin())
	{
		admin.GET("/settings/oplog", h.GetOpLogSetting)
		admin.PUT("/settings/oplog", h.SetOpLogSetting)
		admin.GET("/oplogs", h.ListOpLogs)
	}
}
