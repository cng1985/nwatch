package state

import (
	"github.com/cng1985/nwatch/internal/modules/monitoring/checker"
	"github.com/cng1985/nwatch/internal/modules/monitoring/shared"
	"strconv"
	"strings"
	"time"
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

func (e *Engine) Decide(m *shared.Monitor, r *checker.Result, now time.Time) Decision {
	prev := m.Status
	if prev == "" {
		prev = shared.StatusUnknown
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
		if prev == shared.StatusDown || prev == shared.StatusUnknown {
			if d.Successes >= recoverThreshold {
				d.Status = shared.StatusUp
				d.ClearIncident = true
				if prev == shared.StatusDown {
					d.Events = append(d.Events, EventDraft{
						Type:          shared.EventRecovered,
						OldStatus:     prev,
						NewStatus:     shared.StatusUp,
						Message:       r.Message,
						CloseOpenType: shared.EventDown,
					})
				}
			}
		}
	} else {
		d.Failures++
		d.Successes = 0
		if r.Immediate || d.Failures >= failThreshold {
			if prev != shared.StatusDown {
				d.Status = shared.StatusDown
				started := now
				d.IncidentStartedAt = &started
				// Certificate failures notify through the TLS event only.
				if !(m.Type == shared.TypeTLS && r.TLS != nil && (r.TLS.Status == shared.TLSExpired || r.TLS.Status == shared.TLSInvalid)) {
					d.Events = append(d.Events, EventDraft{
						Type:      shared.EventDown,
						OldStatus: prev,
						NewStatus: shared.StatusDown,
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
				old = shared.TLSNormal
			}
			draft := EventDraft{
				Type:      ev,
				OldStatus: old,
				NewStatus: r.TLS.Status,
				Message:   r.Message,
			}
			if ev == shared.EventTLSRecover {
				draft.CloseOpenType = "TLS"
			}
			d.Events = append(d.Events, draft)
		}
	}
	return d
}

func nextTLSEvent(last string, m *shared.Monitor, info *checker.TLSInfo) (string, string) {
	if info.Status == shared.TLSInvalid {
		if last == "invalid" {
			return "", last
		}
		return shared.EventTLSInvalid, "invalid"
	}
	if info.Status == shared.TLSNormal {
		if last == "" {
			return "", last
		}
		return shared.EventTLSRecover, ""
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
			return shared.EventTLSRecover, ""
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
	case shared.TLSExpired:
		return shared.EventTLSExpire
	case shared.TLSCritical:
		return shared.EventTLSCrit
	case shared.TLSInvalid:
		return shared.EventTLSInvalid
	default:
		return shared.EventTLSWarn
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
