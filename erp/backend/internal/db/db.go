package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cng1985/nwatch/erp/backend/internal/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	if err := gdb.AutoMigrate(
		&models.Product{},
		&models.Inventory{},
		&models.StockMovement{},
		&models.SalesOrder{},
		&models.SalesOrderLine{},
	); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return gdb, nil
}
