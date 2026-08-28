package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"pennypickbackend/internal/config"
	"pennypickbackend/internal/crypto"
	"pennypickbackend/internal/model"
)

const (
	encryptedSuffix = ".enc"
	metaSuffix      = ".enc.meta"
	backupPrefix    = ".bak"
)

// Handle 封装数据库连接与整库加解密的生命周期管理。
//
// 加密模式运行原理：
//   - 落盘形式永远是 <db>.enc（AES-256-GCM 密文），Navicat 等客户端无法打开；
//   - 运行期间解密到系统临时目录的随机命名明文文件，gorm 操作该明文；
//   - 正常退出/周期刷新时把明文加密回写 <db>.enc 并删除明文；
//   - 崩溃后遗留的明文与 meta 文件会在下次启动时被回收并覆盖回 <db>.enc。
type Handle struct {
	db         *gorm.DB
	dbPath     string // 原始数据库路径（如 ./pennypick.db）
	encPath    string // 加密文件路径
	metaPath   string // 记录运行期临时明文路径的文件
	passphrase string
	encrypted  bool   // 当前是否为加密模式运行
	plainMode  bool   // 明文模式（未设置密码，仅开发环境）
	tmpPlainDB string // 运行期临时明文数据库路径
}

// DB 返回 gorm 连接。
func (h *Handle) DB() *gorm.DB { return h.db }

// Open 打开数据库。自动处理：崩溃恢复 → 明文自动迁移加密 → 解密打开。
func Open(cfg *config.Config) (*Handle, error) {
	dbPath := strings.TrimPrefix(cfg.DatabaseURL, "sqlite:///")
	if dbPath == "" {
		return nil, errors.New("empty database url")
	}
	h := &Handle{
		dbPath:     dbPath,
		encPath:    dbPath + encryptedSuffix,
		metaPath:   dbPath + metaSuffix,
		passphrase: cfg.DatabasePass,
	}

	// 1. 已有加密文件：必须提供密码，且先验证密码正确性（防止崩溃恢复用错误密码覆盖 enc）
	encExists, err := fileExists(h.encPath)
	if err != nil {
		return nil, err
	}
	if encExists {
		if h.passphrase == "" {
			return nil, errors.New("数据库已加密：请设置环境变量 PENNYPICK_DB_PASS 提供主密码")
		}
		if _, err := crypto.DecryptFile(h.encPath, h.passphrase); err != nil {
			return nil, fmt.Errorf("unlock database: %w", err)
		}
		// 2. 密码已验证，回收上次异常退出遗留的临时明文并覆盖 enc
		if err := h.recoverCrash(); err != nil {
			return nil, err
		}
		if err := h.decryptAndOpen(); err != nil {
			return nil, err
		}
		return h, nil
	}

	// 3. 无加密文件：先处理崩溃遗留（若上次在迁移中途异常退出，tmp 可能是最新数据）
	if err := h.recoverCrash(); err != nil {
		return nil, err
	}
	if encExists2, err := fileExists(h.encPath); err != nil {
		return nil, err
	} else if encExists2 {
		if h.passphrase == "" {
			return nil, errors.New("数据库已加密：请设置环境变量 PENNYPICK_DB_PASS 提供主密码")
		}
		if err := h.decryptAndOpen(); err != nil {
			return nil, err
		}
		return h, nil
	}

	// 4. 无加密文件也无遗留：明文 db 或全新库
	plainExists, err := fileExists(dbPath)
	if err != nil {
		return nil, err
	}
	if h.passphrase == "" {
		if plainExists {
			log.Printf("[warn] 数据库未加密（未设置 PENNYPICK_DB_PASS），以明文模式运行。如生产使用请务必配置密码。")
		}
		h.plainMode = true
	} else {
		// 有密码：把明文迁移成加密文件（已有明文留 .bak 备份，全新库直接建）
		if plainExists {
			backup := dbPath + backupPrefix + "-" + time.Now().Format("20060102-150405")
			if err := copyFile(dbPath, backup); err != nil {
				return nil, fmt.Errorf("backup plain db: %w", err)
			}
			if err := crypto.EncryptFile(dbPath, h.encPath, h.passphrase); err != nil {
				return nil, err
			}
			_ = os.Remove(dbPath)
			log.Printf("已自动迁移：%s → %s（原明文已备份为 %s）", dbPath, h.encPath, backup)
		} else {
			// 全新库：创建空明文再加密，确保落盘始终只有密文
			if err := os.WriteFile(dbPath, nil, 0o600); err != nil {
				return nil, fmt.Errorf("create empty db: %w", err)
			}
			if err := crypto.EncryptFile(dbPath, h.encPath, h.passphrase); err != nil {
				return nil, err
			}
			_ = os.Remove(dbPath)
			log.Printf("已创建加密数据库：%s", h.encPath)
		}
		if err := h.decryptAndOpen(); err != nil {
			return nil, err
		}
		return h, nil
	}

	// 5. 明文模式直接打开原始文件
	if err := h.openGorm(dbPath); err != nil {
		return nil, err
	}
	return h, nil
}

// decryptAndOpen 解密 .enc 到临时明文文件并打开 gorm，同时写 meta 以便崩溃恢复。
func (h *Handle) decryptAndOpen() error {
	plain, err := crypto.DecryptFile(h.encPath, h.passphrase)
	if err != nil {
		return fmt.Errorf("unlock database: %w", err)
	}
	tmp, err := writeTempPlain(plain)
	if err != nil {
		return err
	}
	h.encrypted = true
	h.tmpPlainDB = tmp
	if err := os.WriteFile(h.metaPath, []byte(tmp), 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write meta file: %w", err)
	}
	if err := h.openGorm(tmp); err != nil {
		_ = os.Remove(tmp)
		_ = os.Remove(h.metaPath)
		return err
	}
	return nil
}

// recoverCrash 处理上次异常退出遗留的临时明文与 meta。
func (h *Handle) recoverCrash() error {
	data, err := os.ReadFile(h.metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read meta file: %w", err)
	}
	tmp := strings.TrimSpace(string(data))
	if tmp == "" {
		_ = os.Remove(h.metaPath)
		return nil
	}
	exists, err := fileExists(tmp)
	if err != nil {
		return err
	}
	if !exists {
		// 临时文件已被清理，丢弃 meta
		_ = os.Remove(h.metaPath)
		return nil
	}
	if h.passphrase == "" {
		return errors.New("检测到上次运行异常退出（存在遗留临时数据），但未配置 PENNYPICK_DB_PASS，无法回收；请设置密码后重试")
	}
	if err := crypto.EncryptFile(tmp, h.encPath, h.passphrase); err != nil {
		return fmt.Errorf("recover crashed data to encrypted db: %w", err)
	}
	_ = os.Remove(tmp)
	_ = os.Remove(h.metaPath)
	log.Printf("已回收上次异常退出遗留的临时数据，并回写加密数据库")
	return nil
}

// openGorm 打开指定路径的 SQLite 并配置连接。
func (h *Handle) openGorm(target string) error {
	db, err := gorm.Open(sqlite.Open(target), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true, // 记录不存在不视为错误，避免日志刷屏
			Colorful:                  false,
		}),
	})
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	// SQLite 单文件：限制连接数避免写锁冲突
	sqlDB.SetMaxOpenConns(1)
	h.db = db
	return nil
}

// Close 正常退出：关闭连接；加密模式下回写 .enc 并清理临时明文与 meta。
func (h *Handle) Close() error {
	var sqlErr error
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			sqlErr = err
		} else if err := sqlDB.Close(); err != nil {
			sqlErr = err
		}
	}
	if !h.encrypted {
		return sqlErr
	}
	if err := h.Flush(); err != nil {
		return err
	}
	_ = os.Remove(h.tmpPlainDB)
	_ = os.Remove(h.metaPath)
	return sqlErr
}

// Flush 把当前临时明文回写到加密文件（原子替换）。加密模式下周期与退出时调用。
func (h *Handle) Flush() error {
	if !h.encrypted || h.tmpPlainDB == "" {
		return nil
	}
	exists, err := fileExists(h.tmpPlainDB)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("tmp plain db missing: %s", h.tmpPlainDB)
	}
	if err := crypto.EncryptFile(h.tmpPlainDB, h.encPath, h.passphrase); err != nil {
		return fmt.Errorf("flush encrypted db: %w", err)
	}
	return nil
}

// AutoFlush 周期回写加密文件，缩小崩溃导致的丢失窗口。stop 关闭后退出。
func (h *Handle) AutoFlush(interval time.Duration, stop <-chan struct{}) {
	if h == nil || !h.encrypted {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := h.Flush(); err != nil {
				log.Printf("[warn] auto flush encrypted db: %v", err)
			}
		}
	}
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
		&model.RecurringBill{},
		&model.MonthlyReport{},
		&model.YearlyReport{},
		&model.BillImport{},
		&model.BillImportItem{},
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

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

// writeTempPlain 把解密后的明文写入系统临时目录的随机命名文件（仅本用户可读写）。
func writeTempPlain(data []byte) (string, error) {
	f, err := os.CreateTemp(os.TempDir(), "pennypick-*.db")
	if err != nil {
		return "", fmt.Errorf("create temp plain db: %w", err)
	}
	name := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(name)
		return "", fmt.Errorf("write temp plain db: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(name)
		return "", fmt.Errorf("close temp plain db: %w", err)
	}
	// 尽力限制为当前用户权限（Windows 上 chmod 影响有限，主要靠临时目录隔离）
	_ = os.Chmod(name, 0o600)
	return name, nil
}
