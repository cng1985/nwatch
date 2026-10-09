package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/cng1985/nwatch/internal/alert"
	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/mailer"
	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/settings"
	"github.com/cng1985/nwatch/internal/state"
	"gorm.io/gorm"
)

type Outcome struct {
	Monitor *model.Monitor
	Result  *checker.Result
	Events  []model.AlertEvent
}

type Processor struct {
	db       *gorm.DB
	checkers *checker.Registry
	engine   *state.Engine
	alerts   *alert.Manager
	metrics  *metric.Aggregator
	settings *settings.Store
	mail     *mailer.Service
}

func NewProcessor(db *gorm.DB, checkers *checker.Registry, engine *state.Engine, alerts *alert.Manager, metrics *metric.Aggregator, store *settings.Store, mail *mailer.Service) *Processor {
	return &Processor{db: db, checkers: checkers, engine: engine, alerts: alerts, metrics: metrics, settings: store, mail: mail}
}

func (p *Processor) Process(ctx context.Context, id uint, reschedule bool) (*Outcome, error) {
	var m model.Monitor
	if err := p.db.Preload("Group").Preload("Notifiers").First(&m, id).Error; err != nil {
		return nil, err
	}
	if !m.Enabled {
		return &Outcome{Monitor: &m}, nil
	}
	chk, err := p.checkers.Must(m.Type)
	if err != nil {
		return nil, err
	}
	result, err := chk.Check(ctx, &m)
	if err != nil || result == nil {
		msg := "检查失败"
		if err != nil {
			msg = err.Error()
		}
		result = &checker.Result{Success: false, Status: "ERROR", Message: msg, CheckedAt: time.Now()}
	}
	if result.CheckedAt.IsZero() {
		result.CheckedAt = time.Now()
	}
	slog.Debug("检查完成", "monitor", m.Name, "type", m.Type, "success", result.Success, "elapsed", result.ResponseTime, "message", result.Message)

	decision := p.engine.Decide(&m, result, result.CheckedAt)
	if decision.Status != m.Status {
		slog.Info("监控状态变化", "monitor", m.Name, "from", m.Status, "to", decision.Status)
	}

	var saved []model.AlertEvent
	err = p.db.Transaction(func(tx *gorm.DB) error {
		meta, _ := json.Marshal(result.Metadata)
		row := model.MonitorCheck{
			MonitorID:    m.ID,
			Success:      result.Success,
			Status:       result.Status,
			StatusCode:   result.StatusCode,
			ResponseTime: result.ResponseTime,
			ErrorMessage: clip(result.Message, 2000),
			Metadata:     string(meta),
			CheckedAt:    result.CheckedAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if m.Type == model.TypeScript {
			run := checker.ScriptRunFromCheck(&m, result)
			if err := tx.Create(&run).Error; err != nil {
				return err
			}
		}
		updates := map[string]any{
			"status":                decision.Status,
			"consecutive_failures":  decision.Failures,
			"consecutive_successes": decision.Successes,
			"last_check_at":         result.CheckedAt,
			"last_response_time":    result.ResponseTime,
			"last_message":          clip(result.Message, 2000),
			"last_status_code":      result.StatusCode,
			"updated_at":            time.Now(),
		}
		if result.Success {
			updates["last_success_at"] = result.CheckedAt
		} else {
			updates["last_failure_at"] = result.CheckedAt
		}
		if decision.IncidentStartedAt != nil {
			updates["incident_started_at"] = *decision.IncidentStartedAt
			updates["notify_stage"] = 0
			updates["next_notify_at"] = decision.IncidentStartedAt.Add(alert.DueOffset(0))
		}
		if decision.ClearIncident {
			updates["incident_started_at"] = gorm.Expr("NULL")
			updates["next_notify_at"] = gorm.Expr("NULL")
			updates["notify_stage"] = 0
		}
		if decision.UpdateCert && decision.Cert != nil {
			cert := decision.Cert
			days := cert.DaysRemaining
			updates["tls_status"] = decision.TLSStatus
			updates["cert_cn"] = cert.CN
			updates["cert_san"] = cert.SAN
			updates["cert_issuer"] = cert.Issuer
			updates["cert_serial"] = cert.Serial
			updates["cert_not_before"] = cert.NotBefore
			updates["cert_not_after"] = cert.NotAfter
			updates["cert_days_remaining"] = days
			updates["last_tls_notified"] = decision.LastTLSNotified
		}
		if reschedule {
			interval := m.Interval
			if interval <= 0 {
				interval = 60
			}
			updates["next_check_at"] = time.Now().Add(time.Duration(interval) * time.Second)
		}
		if err := tx.Model(&model.Monitor{}).Where("id = ?", m.ID).Updates(updates).Error; err != nil {
			return err
		}
		for _, draft := range decision.Events {
			var duration int64
			if m.IncidentStartedAt != nil && (draft.Type == model.EventRecovered || draft.Type == model.EventTLSRecover) {
				duration = int64(result.CheckedAt.Sub(*m.IncidentStartedAt).Seconds())
				if duration < 0 {
					duration = 0
				}
			}
			if draft.CloseOpenType != "" {
				q := tx.Model(&model.AlertEvent{}).Where("monitor_id = ? AND recovered_at IS NULL", m.ID)
				if draft.CloseOpenType == "TLS" {
					q = q.Where("event_type LIKE ?", "TLS_%")
				} else {
					q = q.Where("event_type = ?", draft.CloseOpenType)
				}
				if err := q.Updates(map[string]any{
					"recovered_at": result.CheckedAt,
					"duration":     duration,
				}).Error; err != nil {
					return err
				}
			}
			ev := model.AlertEvent{
				MonitorID:    m.ID,
				MonitorName:  m.Name,
				MonitorType:  m.Type,
				EventType:    draft.Type,
				OldStatus:    draft.OldStatus,
				NewStatus:    draft.NewStatus,
				Target:       checker.FormatTarget(&m),
				Message:      clip(draft.Message, 2000),
				ResponseTime: result.ResponseTime,
				OccurredAt:   result.CheckedAt,
				Duration:     duration,
			}
			if decision.Cert != nil {
				expire := decision.Cert.NotAfter
				days := decision.Cert.DaysRemaining
				ev.CertExpireAt = &expire
				ev.CertDaysLeft = &days
			}
			if err := tx.Create(&ev).Error; err != nil {
				return err
			}
			saved = append(saved, ev)
		}
		return p.metrics.Record(tx, m.ID, result.CheckedAt, result.Success, result.ResponseTime, p.settings.Location())
	})
	if err != nil {
		slog.Error("保存检查结果失败", "monitor", m.Name, "err", err.Error())
		return nil, err
	}
	if m.Type == model.TypeScript {
		checker.PruneScriptRuns(p.db)
	}
	p.dispatchEvents(ctx, &m, decision, saved)
	var fresh model.Monitor
	if err := p.db.Preload("Group").Preload("Notifiers").First(&fresh, m.ID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &Outcome{Monitor: &fresh, Result: result, Events: saved}, nil
}

// 故障通知改由提醒器按 15 秒、45 秒、之后每分钟发送。恢复和仍为正常时的证书提醒立即发送。
func (p *Processor) dispatchEvents(ctx context.Context, m *model.Monitor, decision state.Decision, saved []model.AlertEvent) {
	for _, ev := range saved {
		if decision.Status == model.StatusDown && ev.EventType != model.EventRecovered && ev.EventType != model.EventTLSRecover {
			continue
		}
		if p.alerts != nil {
			p.alerts.Fanout(m.ID, ev)
		}
		if p.mail != nil {
			p.mail.Notify(ctx, mailer.Notice{Event: ev, Failures: decision.Failures})
		}
	}
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
