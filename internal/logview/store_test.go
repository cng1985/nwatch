package logview

import (
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/model"
)

func TestStorePersistsAfterBind(t *testing.T) {
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "logs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store := NewStore()
	logger := slog.New(store.Handler(slog.LevelInfo))
	logger.Info("启动前", "component", "test")
	store.Bind(db)
	logger.Info("启动后", "component", "api")

	var rows []model.AppLog
	if err := db.Order("id asc").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	if rows[0].Message != "启动前" || rows[0].Level != "INFO" || rows[1].Message != "启动后" {
		t.Fatalf("%+v", rows)
	}
	if rows[1].Attrs == "" {
		t.Fatalf("attrs %+v", rows[1])
	}
}
