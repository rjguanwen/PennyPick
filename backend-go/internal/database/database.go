package database

import (
	"fmt"
	"log"
	"strings"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"pennypickbackend/internal/config"
	"pennypickbackend/internal/model"
)

// Open 打开 SQLite 数据库。
func Open(cfg *config.Config) (*gorm.DB, error) {
	dsn := strings.TrimPrefix(cfg.DatabaseURL, "sqlite:///")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}
	// SQLite 单文件：限制连接数避免写锁冲突
	sqlDB.SetMaxOpenConns(1)
	return db, nil
}

// Migrate 自动建表。
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{},
		&model.Category{},
		&model.Account{},
		&model.Bill{},
		&model.Tag{},
		&model.Budget{},
		&model.CategoryBudget{},
		&model.Repayment{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

// InitAdmin 初始化默认用户（个人记账，第一个账号即管理员）。
func InitAdmin(db *gorm.DB, cfg *config.Config) {
	var count int64
	db.Model(&model.User{}).Where("username = ?", cfg.AdminUsername).Count(&count)
	if count > 0 {
		return
	}
	admin := &model.User{
		Username:       cfg.AdminUsername,
		HashedPassword: hashPassword(cfg.AdminPassword),
		Nickname:       cfg.AdminNickname,
		IsActive:       true,
	}
	if err := db.Create(admin).Error; err != nil {
		log.Printf("init admin: %v", err)
		return
	}
	model.SeedDefaultCategories(db, admin.ID)
	model.SeedDefaultAccounts(db, admin.ID)
	log.Printf("已创建默认用户 %s，并初始化预置分类/账户", cfg.AdminUsername)
}

func hashPassword(password string) string {
	hashed, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashed)
}
