package state

import (
	"strconv"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/checker"
	"github.com/cng1985/nwatch/internal/model"
)

type EventDraft struct {
	Type          string
	OldStatus     string
	NewStatus     string
	Message       string
	CloseOpenType string
}

type Decision struct {
	Status            string
	TLSStatus         string
	Failures          int
	Successes         int
	IncidentStartedAt *time.Time
	ClearIncident     bool
	LastTLSNotified   string
	Events            []EventDraft
	UpdateCert        bool
	Cert              *checker.TLSInfo
}

type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

func (e *Engine) Decide(m *model.Monitor, r *checker.Result, now time.Time) Decision {
	prev := m.Status
	if prev == "" {
		prev = model.StatusUnknown
	}
	d := Decision{
		Status:          prev,
		TLSStatus:       m.TLSStatus,
		Failures:        m.ConsecutiveFailures,
		Successes:       m.ConsecutiveSuccesses,
		LastTLSNotified: m.LastTLSNotified,
	}

	failThreshold := m.FailureThreshold
	if failThreshold <= 0 {
		failThreshold = 3
	}
	recoverThreshold := m.RecoveryThreshold
	if recoverThreshold <= 0 {
		recoverThreshold = 1
	}

	if r.Success {
		d.Successes++
		if d.Successes > 1_000_000 {
			d.Successes = recoverThreshold
		}
		d.Failures = 0
		if prev == model.StatusDown || prev == model.StatusUnknown {
			if d.Successes >= recoverThreshold {
				d.Status = model.StatusUp
				d.ClearIncident = true
				if prev == model.StatusDown {
					d.Events = append(d.Events, EventDraft{
						Type:          model.EventRecovered,
						OldStatus:     prev,
						NewStatus:     model.StatusUp,
						Message:       r.Message,
						CloseOpenType: model.EventDown,
					})
				}
			}
		}
	} else {
		d.Failures++
		d.Successes = 0
		if r.Immediate || d.Failures >= failThreshold {
			if prev != model.StatusDown {
				d.Status = model.StatusDown
				started := now
				d.IncidentStartedAt = &started
				// Certificate failures notify through the TLS event only.
				if !(m.Type == model.TypeTLS && r.TLS != nil && (r.TLS.Status == model.TLSExpired || r.TLS.Status == model.TLSInvalid)) {
					d.Events = append(d.Events, EventDraft{
						Type:      model.EventDown,
						OldStatus: prev,
						NewStatus: model.StatusDown,
						Message:   r.Message,
					})
				}
			}
		}
	}

	if r.TLS != nil {
		d.UpdateCert = true
		d.Cert = r.TLS
		d.TLSStatus = r.TLS.Status
		ev, notified := nextTLSEvent(m.LastTLSNotified, m, r.TLS)
		d.LastTLSNotified = notified
		if ev != "" {
			old := m.TLSStatus
			if old == "" {
				old = model.TLSNormal
			}
			draft := EventDraft{
				Type:      ev,
				OldStatus: old,
				NewStatus: r.TLS.Status,
				Message:   r.Message,
			}
			if ev == model.EventTLSRecover {
				draft.CloseOpenType = "TLS"
			}
			d.Events = append(d.Events, draft)
		}
	}
	return d
}

func nextTLSEvent(last string, m *model.Monitor, info *checker.TLSInfo) (string, string) {
	if info.Status == model.TLSInvalid {
		if last == "invalid" {
			return "", last
		}
		return model.EventTLSInvalid, "invalid"
	}
	if info.Status == model.TLSNormal {
		if last == "" {
			return "", last
		}
		return model.EventTLSRecover, ""
	}

	thresholds := parseThresholds(m.TLSNotifyDays)
	if len(thresholds) == 0 {
		thresholds = []int{30, 14, 7, 3, 1, 0}
	}
	hasZero := false
	for _, th := range thresholds {
		if th == 0 {
			hasZero = true
			break
		}
	}
	if !hasZero {
		thresholds = append(thresholds, 0)
	}
	matched := -1
	for _, th := range thresholds {
		if info.DaysRemaining <= th && (matched == -1 || th < matched) {
			matched = th
		}
	}
	if matched == -1 {
		if last != "" {
			return model.EventTLSRecover, ""
		}
		return "", ""
	}
	token := strconv.Itoa(matched)
	prev, err := strconv.Atoi(last)
	// A smaller threshold is more urgent. Alert only when a tighter one is crossed.
	// If the certificate was renewed into a looser window, remember it without alerting.
	if err != nil || last == "invalid" || matched < prev {
		return tlsEventType(info.Status), token
	}
	return "", token
}

func tlsEventType(status string) string {
	switch status {
	case model.TLSExpired:
		return model.EventTLSExpire
	case model.TLSCritical:
		return model.EventTLSCrit
	case model.TLSInvalid:
		return model.EventTLSInvalid
	default:
		return model.EventTLSWarn
	}
}

func parseThresholds(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}
