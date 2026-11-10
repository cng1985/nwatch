// Package database 提供数据库连接、连接池与生命周期管理。
// 四库支持：sqlite（默认，已验证）、mysql、postgres、sqlserver（连接能力已具备，兼容性见 docs 中支持矩阵）。
// 迁移与初始化由 app/bootstrap 编排，构造函数只负责打开连接。
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cng1985/nwatch/internal/platform/config"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Options 是打开连接所需的全部配置。
type Options struct {
	Driver          string
	DSN             string
	Path            string // sqlite 专用，已解析为绝对路径
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ForeignKeys     bool          // sqlite 外键开关
	BusyTimeout     time.Duration // sqlite 忙等待
	JournalMode     string        // sqlite 日志模式
}

// OptionsFromConfig 从进程配置构造连接选项。
func OptionsFromConfig(cfg *config.Config) Options {
	db := cfg.Database
	return Options{
		Driver:          db.Driver,
		DSN:             db.DSN,
		Path:            db.Path,
		MaxOpenConns:    db.MaxOpenConns,
		MaxIdleConns:    db.MaxIdleConns,
		ConnMaxLifetime: db.ConnMaxLifetime,
		ForeignKeys:     db.SQLite.ForeignKeys,
		BusyTimeout:     db.SQLite.BusyTimeout,
		JournalMode:     db.SQLite.JournalMode,
	}
}

// Open 按驱动打开数据库并设置连接池。显式连接失败必须返回错误，不静默回退。
func Open(opts Options) (*gorm.DB, error) {
	var dial gorm.Dialector
	switch opts.Driver {
	case "sqlite":
		if dir := filepath.Dir(opts.Path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("创建数据目录: %w", err)
			}
		}
		dial = sqlite.Open(opts.Path)
	case "mysql":
		dial = mysql.Open(opts.DSN)
	case "postgres", "pgsql":
		dial = postgres.Open(opts.DSN)
	case "sqlserver":
		dial = sqlserver.Open(opts.DSN)
	default:
		return nil, fmt.Errorf("未知数据库驱动: %s", opts.Driver)
	}
	db, err := gorm.Open(dial, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(opts.ConnMaxLifetime)
	if opts.Driver == "sqlite" {
		if err := applySQLitePragmas(db, opts); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}
	return db, nil
}

// applySQLitePragmas 逐连接落实外键与忙等待；连接池实际使用的连接都会带上这些设置。
func applySQLitePragmas(db *gorm.DB, opts Options) error {
	mode := opts.JournalMode
	if mode == "" {
		mode = "WAL"
	}
	pragmas := []string{
		"PRAGMA journal_mode=" + mode + ";",
		"PRAGMA synchronous=NORMAL;",
		fmt.Sprintf("PRAGMA busy_timeout=%d;", opts.BusyTimeout.Milliseconds()),
	}
	if opts.ForeignKeys {
		pragmas = append(pragmas, "PRAGMA foreign_keys=ON;")
	}
	for _, p := range pragmas {
		if err := db.Exec(p).Error; err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
	}
	return nil
}

// Close 关闭连接池。
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
