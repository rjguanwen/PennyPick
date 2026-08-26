package model

import (
	"time"
)

// 导入平台
const (
	PlatformWechat = "wechat" // 微信
	PlatformAlipay = "alipay" // 支付宝
)

// 导入任务状态
const (
	ImportStatusPending   = "pending"   // 解析中/待确认
	ImportStatusCompleted = "completed" // 已完成
	ImportStatusFailed    = "failed"    // 失败
)

// 导入明细状态
const (
	ImportItemImported = "imported" // 已导入为账单
	ImportItemSkipped  = "skipped"  // 已跳过
)

// BillImport 账单导入任务（一次上传解析对应一个任务）。
type BillImport struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index;not null" json:"user_id"`
	Platform      string    `gorm:"size:16;not null" json:"platform"` // wechat / alipay
	FileName      string    `gorm:"size:255" json:"file_name"`
	TotalCount    int       `json:"total_count"`    // 解析出的总条数（含过滤/重复）
	ImportedCount int       `json:"imported_count"` // 实际导入账单数
	SkippedCount  int       `json:"skipped_count"`  // 跳过条数（过滤 + 重复 + 未选中）
	Status        string    `gorm:"size:16;default:pending" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// BillImportItem 导入明细（保留每条原始记录与订单号，用于历史追溯与防重复）。
type BillImportItem struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ImportID        uint      `gorm:"index;not null" json:"import_id"`
	BillID          *uint     `gorm:"index" json:"bill_id"` // 导入生成的账单，跳过的为空
	PlatformOrderNo string    `gorm:"size:128;index;not null" json:"platform_order_no"`
	OccurredAt      time.Time `json:"occurred_at"`
	Amount          float64   `json:"amount"`
	Type            string    `gorm:"size:8" json:"type"` // expense / income（被过滤的为空）
	Counterparty    string    `gorm:"size:255" json:"counterparty"`
	Note            string    `gorm:"size:255" json:"note"`
	Status          string    `gorm:"size:16;default:imported" json:"status"` // imported / skipped
	SkipReason      string    `gorm:"size:255" json:"skip_reason"`
	RawData         string    `gorm:"type:text" json:"-"` // 原始行 JSON（便于溯源）
	CreatedAt       time.Time `json:"created_at"`
}

// ImportItem 解析出的待确认导入项（用于前后端交互，不落库）。
type ImportItem struct {
	PlatformOrderNo string  `json:"platform_order_no"`
	OccurredAt      string  `json:"occurred_at"`
	Amount          float64 `json:"amount"`
	Type            string  `json:"type"` // expense / income
	Counterparty    string  `json:"counterparty"`
	Note            string  `json:"note"`
	PayWay          string  `json:"pay_way"` // 支付方式（用于自动匹配账户）
	CategoryID      uint    `json:"category_id"` // 用户确认的分类（0 表示交给后端智能兜底）
	AccountID       uint    `json:"account_id"`  // 用户确认的账户（0 表示用默认账户）
	IsDuplicate     bool    `json:"is_duplicate"`
	DuplicateWay    string  `json:"duplicate_way"` // order_no / fuzzy
	IsFiltered      bool    `json:"is_filtered"`
	FilterReason    string  `json:"filter_reason"`
	Selected        bool    `json:"selected"` // 前端确认后回传
	RawData         string  `json:"-"`        // 原始行 JSON
}

// ParseResult 解析结果汇总。
type ParseResult struct {
	Items      []ImportItem `json:"items"`
	TotalCount int          `json:"total_count"`
	Expenses   int          `json:"expenses"`
	Incomes    int          `json:"incomes"`
	Duplicates int          `json:"duplicates"`
	Filtered   int          `json:"filtered"`
}
