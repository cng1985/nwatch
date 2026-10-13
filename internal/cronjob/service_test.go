//go:build unix

package cronjob

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cng1985/nwatch/internal/config"
	"github.com/cng1985/nwatch/internal/database"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"gorm.io/gorm"
)

func TestScanRunsDueJob(t *testing.T) {
	svc, db := testService(t)
	now := svc.Now()
	past := now.Add(-time.Minute)
	job := model.CronJob{
		Name: "echo", Cron: "* * * * *", Command: "echo cron-ok",
		Timeout: 5, Enabled: true, NextRunAt: &past,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	svc.scan()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var row model.CronRun
		err := db.Where("job_id = ?", job.ID).First(&row).Error
		if err != nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if !row.Success || row.Trigger != model.CronTriggerSchedule || !strings.Contains(row.Stdout, "cron-ok") {
			t.Fatalf("%+v", row)
		}
		var saved model.CronJob
		if err := db.First(&saved, job.ID).Error; err != nil {
			t.Fatal(err)
		}
		if saved.LastStatus == model.CronRunning {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if saved.LastStatus != model.CronSuccess || saved.NextRunAt == nil || !saved.NextRunAt.After(now) {
			t.Fatalf("%+v", saved)
		}
		return
	}
	t.Fatal("执行记录没有写入")
}

func TestRunSkipsOverlap(t *testing.T) {
	svc, db := testService(t)
	job := model.CronJob{Name: "sleep", Cron: "* * * * *", Command: "sleep 2", Timeout: 10, Enabled: true}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := svc.Run(job.ID, model.CronTriggerSchedule)
		done <- err
	}()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !svc.Busy(job.ID) {
		time.Sleep(5 * time.Millisecond)
	}
	if !svc.Busy(job.ID) {
		t.Fatal("任务没有进入执行")
	}
	if _, err := svc.Run(job.ID, model.CronTriggerManual); !errors.Is(err, ErrBusy) {
		t.Fatalf("overlap err = %v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&model.CronRun{}).Where("job_id = ?", job.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("runs = %d", n)
	}
}

func TestDisabledJobIsNotClaimed(t *testing.T) {
	svc, db := testService(t)
	past := svc.Now().Add(-time.Minute)
	job := model.CronJob{
		Name: "off", Cron: "* * * * *", Command: "echo no",
		Timeout: 5, Enabled: false, NextRunAt: &past,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	svc.scan()
	var saved model.CronJob
	if err := db.First(&saved, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.NextRunAt == nil || !saved.NextRunAt.Before(time.Now()) {
		t.Fatalf("disabled job was claimed: %v", saved.NextRunAt)
	}
	saved, err := svc.SetEnabled(job.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if saved.NextRunAt == nil || !saved.NextRunAt.After(time.Now().Add(-time.Second)) {
		t.Fatalf("next = %v", saved.NextRunAt)
	}
}

func testService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "cron.db")
	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(db) })
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	store, err := settings.New(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(db, store, time.Hour, nil)
	t.Cleanup(svc.Stop)
	return svc, db
}
