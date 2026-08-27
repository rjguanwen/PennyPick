package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"pennypickbackend/internal/config"
	"pennypickbackend/internal/crypto"
)

func openRaw(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	return db
}

// TestEncryptLifecycle 覆盖：明文→自动迁移加密→关闭回写→重新解密打开→错误密码。
func TestEncryptLifecycle(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// 1. 先用明文模式创建数据库并写入数据（模拟已有业务数据）
	plainCfg := &config.Config{DatabaseURL: dbPath}
	h0, err := Open(plainCfg)
	if err != nil {
		t.Fatalf("open plain: %v", err)
	}
	if h0.plainMode == false {
		t.Fatal("expected plain mode")
	}
	db := h0.DB()
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO t (name) VALUES ('hello')").Error; err != nil {
		t.Fatal(err)
	}
	if err := h0.Close(); err != nil {
		t.Fatalf("close plain: %v", err)
	}

	// 2. 带密码打开：应自动迁移出 .enc，删除明文
	cfg := &config.Config{DatabaseURL: dbPath, DatabasePass: "test-pass"}
	h1, err := Open(cfg)
	if err != nil {
		t.Fatalf("open with pass (migrate): %v", err)
	}
	if !h1.encrypted {
		t.Fatal("expected encrypted mode after migration")
	}
	if _, err := os.Stat(dbPath + encryptedSuffix); err != nil {
		t.Fatalf("enc file missing: %v", err)
	}
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatal("plain db should be removed after migration")
	}
	// 明文备份应存在
	baks, _ := filepath.Glob(dbPath + backupPrefix + "-*")
	if len(baks) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(baks))
	}
	// 数据应可读
	var name string
	if err := h1.DB().Raw("SELECT name FROM t LIMIT 1").Scan(&name).Error; err != nil {
		t.Fatalf("read after migration: %v", err)
	}
	if name != "hello" {
		t.Fatalf("unexpected name: %s", name)
	}
	// 运行期临时明文应存在于系统临时目录
	if h1.tmpPlainDB == "" {
		t.Fatal("tmpPlainDB empty")
	}
	if _, err := os.Stat(h1.tmpPlainDB); err != nil {
		t.Fatalf("tmp plain missing: %v", err)
	}
	if _, err := os.Stat(h1.metaPath); err != nil {
		t.Fatalf("meta missing: %v", err)
	}
	if err := h1.Close(); err != nil {
		t.Fatalf("close encrypted: %v", err)
	}
	// 关闭后临时明文与 meta 应被清理
	if _, err := os.Stat(h1.tmpPlainDB); !os.IsNotExist(err) {
		t.Fatal("tmp plain should be removed after close")
	}
	if _, err := os.Stat(h1.metaPath); !os.IsNotExist(err) {
		t.Fatal("meta should be removed after close")
	}
	// enc 应可被直接解密
	if _, err := crypto.DecryptFile(dbPath+encryptedSuffix, "test-pass"); err != nil {
		t.Fatalf("decrypt enc: %v", err)
	}

	// 3. 重新打开（解密模式）：数据应完整
	h2, err := Open(cfg)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	name = ""
	if err := h2.DB().Raw("SELECT name FROM t LIMIT 1").Scan(&name).Error; err != nil {
		t.Fatalf("read after reopen: %v", err)
	}
	if name != "hello" {
		t.Fatalf("unexpected name after reopen: %s", name)
	}
	if err := h2.Close(); err != nil {
		t.Fatalf("close reopen: %v", err)
	}

	// 4. 错误密码应报错
	badCfg := &config.Config{DatabaseURL: dbPath, DatabasePass: "wrong-pass"}
	if _, err := Open(badCfg); err == nil {
		t.Fatal("expected error with wrong password")
	}
	// 5. 有 enc 但未提供密码应报错
	noPassCfg := &config.Config{DatabaseURL: dbPath}
	if _, err := Open(noPassCfg); err == nil {
		t.Fatal("expected error when enc exists but no password")
	}
}

// TestCrashRecovery 覆盖：模拟异常退出遗留临时明文 + meta，下次启动应回收并回写 enc。
func TestCrashRecovery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	cfg := &config.Config{DatabaseURL: dbPath, DatabasePass: "pass"}

	// 正常跑一轮：生成 enc
	h1, err := Open(cfg)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db := h1.DB()
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := h1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 模拟崩溃：解密 enc → 写到遗留临时文件 → 追加一行数据 → 写 meta 指向它
	plain, err := crypto.DecryptFile(dbPath+encryptedSuffix, "pass")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	stale := filepath.Join(dir, "stale-tmp.db")
	if err := os.WriteFile(stale, plain, 0o600); err != nil {
		t.Fatal(err)
	}
	gh := openRaw(t, stale)
	if err := gh.Exec("INSERT INTO t (name) VALUES ('crash-data')").Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := gh.DB()
	sqlDB.Close()
	if err := os.WriteFile(dbPath+metaSuffix, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}

	// 再次打开：应触发崩溃恢复，用遗留 tmp 覆盖 enc
	h2, err := Open(cfg)
	if err != nil {
		t.Fatalf("reopen after crash: %v", err)
	}
	var n int64
	if err := h2.DB().Raw("SELECT COUNT(*) FROM t").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row (crash data recovered), got %d", n)
	}
	// 遗留临时文件应被清理；meta 应指向新的临时明文（而非遗留路径）
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale tmp should be removed after recovery")
	}
	metaData, err := os.ReadFile(h2.metaPath)
	if err != nil {
		t.Fatalf("new meta missing: %v", err)
	}
	if strings.TrimSpace(string(metaData)) == stale {
		t.Fatal("meta should point to new tmp, not the stale path")
	}
	if _, err := os.Stat(h2.tmpPlainDB); err != nil {
		t.Fatalf("new tmp missing: %v", err)
	}
	if err := h2.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// enc 应已包含 crash-data
	plain2, err := crypto.DecryptFile(dbPath+encryptedSuffix, "pass")
	if err != nil {
		t.Fatal(err)
	}
	checkPath := filepath.Join(dir, "check.db")
	if err := os.WriteFile(checkPath, plain2, 0o600); err != nil {
		t.Fatal(err)
	}
	gh2 := openRaw(t, checkPath)
	var cnt int64
	if err := gh2.Raw("SELECT COUNT(*) FROM t").Scan(&cnt).Error; err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Fatalf("enc after recovery should contain crash data, got %d rows", cnt)
	}
	sqlDB2, _ := gh2.DB()
	sqlDB2.Close()
}

// TestFreshDBWithPass 覆盖：全新库 + 密码 → 直接以加密形式落盘。
func TestFreshDBWithPass(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "fresh.db")
	cfg := &config.Config{DatabaseURL: dbPath, DatabasePass: "fresh-pass"}

	h, err := Open(cfg)
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	if !h.encrypted {
		t.Fatal("expected encrypted mode for fresh db with pass")
	}
	// 运行期明文 dbPath 不应存在
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatal("plain dbPath should not exist in encrypted mode")
	}
	if err := Migrate(h.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// 只有 .enc 存在
	if _, err := os.Stat(dbPath + encryptedSuffix); err != nil {
		t.Fatalf("enc missing: %v", err)
	}
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatal("plain dbPath should still not exist")
	}
}
