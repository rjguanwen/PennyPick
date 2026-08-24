package model

import (
	"math"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// 账单类型
const (
	TypeExpense = "expense" // 支出
	TypeIncome  = "income"  // 收入
)

// MaxBillTags 每条账单最多可添加的标签数。
const MaxBillTags = 8

// User 用户（个人记账：每个用户的数据相互隔离）。
type User struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Username       string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	HashedPassword string    `gorm:"size:255;not null" json:"-"`
	Nickname       string    `gorm:"size:64" json:"nickname"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// CheckPassword 校验密码。
func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.HashedPassword), []byte(password)) == nil
}

// SetPassword 更新密码。
func (u *User) SetPassword(password string) {
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	u.HashedPassword = string(hashed)
}

// HashPassword 生成密码哈希。
func HashPassword(password string) string {
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashed)
}

// Category 分类。
type Category struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Name      string    `gorm:"size:32;not null" json:"name"`
	Type      string    `gorm:"size:8;not null;index" json:"type"` // expense / income
	Icon      string    `gorm:"size:32" json:"icon"`
	Color     string    `gorm:"size:16" json:"color"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`

	RecentCount int `gorm:"-" json:"recent_count"` // 近30天使用次数（接口计算）
}

// Account 账户。
type Account struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Name      string    `gorm:"size:32;not null" json:"name"`
	Icon      string    `gorm:"size:32" json:"icon"`
	IsCredit  bool      `gorm:"default:false" json:"is_credit"` // 是否先用后还（信用）账户
	RepayDay  int       `gorm:"default:25" json:"repay_day"`    // 每月还款截止日（1-28）
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// 还款状态
const (
	RepayStatusFull    = "full"    // 已全额还清
	RepayStatusPartial = "partial" // 部分还款（未彻底还清）
)

// Repayment 账户月度还款记录（标记某账户某月已还款）。
type Repayment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uk_user_acc_month;not null" json:"user_id"`
	AccountID uint      `gorm:"uniqueIndex:uk_user_acc_month;not null" json:"account_id"`
	Month     string    `gorm:"size:7;uniqueIndex:uk_user_acc_month;not null" json:"month"` // YYYY-MM
	Amount    float64   `json:"amount"`                                                    // 实际还款金额
	Status    string    `gorm:"size:8;default:full" json:"status"`                         // full 全额 / partial 部分
	Note      string    `gorm:"size:255" json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// Tag 账单标签（标签库属于用户，可被多条账单复用）。
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Name      string    `gorm:"size:16;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Bill 账单（一笔支出/收入）。
type Bill struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	CategoryID uint      `gorm:"index;not null" json:"category_id"`
	AccountID  *uint     `gorm:"index" json:"account_id"`
	Type       string    `gorm:"size:8;not null" json:"type"` // expense / income
	Amount     float64   `gorm:"not null" json:"amount"`
	Note       string    `gorm:"size:255" json:"note"`
	OccurredAt DateTime  `json:"occurred_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	Category *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Account  *Account  `gorm:"foreignKey:AccountID" json:"account,omitempty"`
	Tags     []Tag     `gorm:"many2many:bill_tags;" json:"tags,omitempty"`
}

// Budget 月度总预算（按月设置预警）。
type Budget struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex:uk_user_month;not null" json:"user_id"`
	Month       string    `gorm:"size:7;uniqueIndex:uk_user_month;not null" json:"month"` // YYYY-MM
	Amount      float64   `gorm:"not null" json:"amount"`
	WarnPercent float64   `gorm:"default:80" json:"warn_percent"` // 预警阈值百分比
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CategoryBudget 分类预算（某月某个支出分类的预算，独立预警）。
type CategoryBudget struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"uniqueIndex:uk_user_month_cat;not null" json:"user_id"`
	Month       string    `gorm:"size:7;uniqueIndex:uk_user_month_cat;not null" json:"month"` // YYYY-MM
	CategoryID  uint      `gorm:"uniqueIndex:uk_user_month_cat;not null" json:"category_id"`
	Amount      float64   `gorm:"not null" json:"amount"`
	WarnPercent float64   `gorm:"default:80" json:"warn_percent"` // 预警阈值百分比
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Category *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
}

// CategoryPreset 预置分类定义。
type CategoryPreset struct {
	Name  string
	Type  string
	Icon  string
	Color string
}

var expensePresets = []CategoryPreset{
	{"餐饮", TypeExpense, "Dish", "#FF6B35"},
	{"交通", TypeExpense, "Van", "#409EFF"},
	{"购物", TypeExpense, "ShoppingBag", "#E6607A"},
	{"居住", TypeExpense, "House", "#7E57C2"},
	{"娱乐", TypeExpense, "Film", "#8A2BE2"},
	{"医疗", TypeExpense, "FirstAidKit", "#F56C6C"},
	{"教育", TypeExpense, "Reading", "#67C23A"},
	{"人情", TypeExpense, "Present", "#FF9F43"},
	{"旅行", TypeExpense, "Suitcase", "#00B4D8"},
	{"其他", TypeExpense, "More", "#909399"},
}

var incomePresets = []CategoryPreset{
	{"工资", TypeIncome, "Wallet", "#67C23A"},
	{"奖金", TypeIncome, "Trophy", "#F7BA2A"},
	{"理财", TypeIncome, "TrendCharts", "#409EFF"},
	{"兼职", TypeIncome, "Coin", "#E6A23C"},
	{"退款", TypeIncome, "Refresh", "#00B4D8"},
	{"其他", TypeIncome, "More", "#909399"},
}

var accountPresets = []CategoryPreset{
	{"现金", TypeExpense, "Money", "#67C23A"},
	{"微信", TypeExpense, "ChatDotRound", "#07C160"},
	{"支付宝", TypeExpense, "Wallet", "#1677FF"},
	{"银行卡", TypeExpense, "CreditCard", "#F56C6C"},
}

// SeedDefaultCategories 为新用户初始化预置分类。
func SeedDefaultCategories(db *gorm.DB, userID uint) {
	all := make([]CategoryPreset, 0, len(expensePresets)+len(incomePresets))
	all = append(all, expensePresets...)
	all = append(all, incomePresets...)
	for i, p := range all {
		db.Create(&Category{
			UserID:    userID,
			Name:      p.Name,
			Type:      p.Type,
			Icon:      p.Icon,
			Color:     p.Color,
			SortOrder: i,
		})
	}
}

// SeedDefaultAccounts 为新用户初始化预置账户。
func SeedDefaultAccounts(db *gorm.DB, userID uint) {
	for i, p := range accountPresets {
		db.Create(&Account{
			UserID:    userID,
			Name:      p.Name,
			Icon:      p.Icon,
			SortOrder: i,
		})
	}
}

// Round2 金额保留两位小数。
func Round2(v float64) float64 {
	return math.Round(v*100) / 100
}
