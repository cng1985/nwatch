package dashboard

import (
	"time"

	"github.com/cng1985/nwatch/internal/metric"
	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

type TrendPoint struct {
	Time time.Time `json:"time"`
	Avg  int       `json:"avg"`
}

type Trend struct {
	Range   string       `json:"range"`
	Avg     int          `json:"avg"`
	Min     int          `json:"min"`
	Max     int          `json:"max"`
	Delta   *float64     `json:"delta"`
	Success int          `json:"success"`
	Failure int          `json:"failure"`
	Points  []TrendPoint `json:"points"`
}

type Overview struct {
	ID               uint       `json:"id"`
	Name             string     `json:"name"`
	Type             string     `json:"type"`
	URL              string     `json:"url"`
	Host             string     `json:"host"`
	Port             int        `json:"port"`
	Status           string     `json:"status"`
	TLSStatus        string     `json:"tlsStatus"`
	LastResponseTime int        `json:"lastResponseTime"`
	LastCheckAt      *time.Time `json:"lastCheckAt"`
	Availability24h  float64    `json:"availability24h"`
	HasAvailability  bool       `json:"hasAvailability"`
	GroupName        string     `json:"groupName"`
	GroupID          *uint      `json:"groupId"`
}

type Summary struct {
	Total               int                `json:"total"`
	Up                  int                `json:"up"`
	Down                int                `json:"down"`
	Paused              int                `json:"paused"`
	Unknown             int                `json:"unknown"`
	AddedSinceYesterday int                `json:"addedSinceYesterday"`
	Availability24h     float64            `json:"availability24h"`
	HasAvailability     bool               `json:"hasAvailability"`
	Success24h          int                `json:"success24h"`
	Failure24h          int                `json:"failure24h"`
	OpenAlerts          int                `json:"openAlerts"`
	RecentEvents        []model.AlertEvent `json:"recentEvents"`
	Problems            []model.Monitor    `json:"problems"`
	ExpiringCerts       []model.Monitor    `json:"expiringCerts"`
	Overview            []Overview         `json:"overview"`
	Trend               Trend              `json:"trend"`
}

func Query(db *gorm.DB, rangeName string) (Summary, error) {
	var out Summary
	var monitors []model.Monitor
	if err := db.Preload("Group").Order(StatusOrder()).Find(&monitors).Error; err != nil {
		return out, err
	}
	out.Total = len(monitors)
	for _, m := range monitors {
		switch m.Status {
		case model.StatusUp:
			out.Up++
		case model.StatusDown:
			out.Down++
		case model.StatusPaused:
			out.Paused++
		default:
			out.Unknown++
		}
		if isProblem(m) {
			out.OpenAlerts++
			if len(out.Problems) < 8 {
				out.Problems = append(out.Problems, m)
			}
		}
	}
	if err := db.Where("type = ?", model.TypeTLS).
		Order("CASE WHEN cert_days_remaining IS NULL THEN 1 ELSE 0 END, cert_days_remaining ASC").
		Limit(8).
		Find(&out.ExpiringCerts).Error; err != nil {
		return out, err
	}
	if err := db.Order("occurred_at desc").Limit(8).Find(&out.RecentEvents).Error; err != nil {
		return out, err
	}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var added int64
	if err := db.Model(&model.Monitor{}).Where("created_at >= ?", dayStart).Count(&added).Error; err != nil {
		return out, err
	}
	out.AddedSinceYesterday = int(added)
	avail, err := metric.AvailabilityByMonitor(db, now.Add(-24*time.Hour), model.BucketHour)
	if err != nil {
		return out, err
	}
	var success, totalChecks int
	for _, item := range avail {
		success += item.Success
		totalChecks += item.Total
	}
	if totalChecks > 0 {
		out.Availability24h = float64(success) / float64(totalChecks) * 100
		out.HasAvailability = true
		out.Success24h = success
		out.Failure24h = totalChecks - success
	}
	out.Overview = make([]Overview, 0, 8)
	for _, m := range monitors {
		if len(out.Overview) >= 8 {
			break
		}
		item := Overview{
			ID: m.ID, Name: m.Name, Type: m.Type, URL: m.URL, Host: m.Host, Port: m.Port,
			Status: m.Status, TLSStatus: m.TLSStatus, LastResponseTime: m.LastResponseTime, LastCheckAt: m.LastCheckAt,
		}
		item.GroupID = m.GroupID
		if m.Group != nil {
			item.GroupName = m.Group.Name
		}
		if stat, ok := avail[m.ID]; ok && stat.Total > 0 {
			item.HasAvailability = true
			item.Availability24h = float64(stat.Success) / float64(stat.Total) * 100
		}
		out.Overview = append(out.Overview, item)
	}
	trend, err := buildTrend(db, now, rangeName)
	if err != nil {
		return out, err
	}
	out.Trend = trend
	if out.Problems == nil {
		out.Problems = []model.Monitor{}
	}
	if out.ExpiringCerts == nil {
		out.ExpiringCerts = []model.Monitor{}
	}
	if out.RecentEvents == nil {
		out.RecentEvents = []model.AlertEvent{}
	}
	return out, nil
}

func buildTrend(db *gorm.DB, now time.Time, rangeName string) (Trend, error) {
	bucket := model.BucketHour
	span := 24 * time.Hour
	switch rangeName {
	case "7d":
		span = 7 * 24 * time.Hour
	case "30d":
		span = 30 * 24 * time.Hour
		bucket = model.BucketDay
	default:
		rangeName = "24h"
	}
	from := now.Add(-span)
	prevFrom := from.Add(-span)
	current, err := windowStats(db, from, now, bucket)
	if err != nil {
		return Trend{}, err
	}
	previous, err := windowStats(db, prevFrom, from, bucket)
	if err != nil {
		return Trend{}, err
	}
	trend := Trend{
		Range: rangeName, Avg: current.avg, Min: current.min, Max: current.max,
		Success: current.success, Failure: current.failure, Points: current.points,
	}
	if trend.Points == nil {
		trend.Points = []TrendPoint{}
	}
	if previous.avg > 0 && current.total > 0 {
		delta := (float64(current.avg) - float64(previous.avg)) / float64(previous.avg) * 100
		trend.Delta = &delta
	}
	return trend, nil
}

type window struct {
	avg, min, max, success, failure, total int
	points                                 []TrendPoint
}

func windowStats(db *gorm.DB, from, to time.Time, bucket string) (window, error) {
	var rows []model.MonitorMetric
	if err := db.Where("bucket_type = ? AND bucket_time >= ? AND bucket_time < ?", bucket, from, to).
		Order("bucket_time asc").Find(&rows).Error; err != nil {
		return window{}, err
	}
	type acc struct {
		at     time.Time
		sum    int64
		total  int
		min    int
		max    int
		hasMin bool
	}
	order := []int64{}
	grouped := map[int64]*acc{}
	var out window
	var weighted int64
	for _, row := range rows {
		key := row.BucketTime.Unix()
		item := grouped[key]
		if item == nil {
			item = &acc{at: row.BucketTime}
			grouped[key] = item
			order = append(order, key)
		}
		item.total += row.Total
		item.sum += row.SumResponseTime
		if row.MaxResponseTime > item.max {
			item.max = row.MaxResponseTime
		}
		if row.MinResponseTime > 0 && (!item.hasMin || row.MinResponseTime < item.min) {
			item.min = row.MinResponseTime
			item.hasMin = true
		}
		out.success += row.Success
		out.failure += row.Failure
		out.total += row.Total
		weighted += row.SumResponseTime
		if row.MaxResponseTime > out.max {
			out.max = row.MaxResponseTime
		}
		if row.MinResponseTime > 0 && (out.min == 0 || row.MinResponseTime < out.min) {
			out.min = row.MinResponseTime
		}
	}
	if out.total > 0 {
		out.avg = int(weighted / int64(out.total))
	}
	for _, key := range order {
		item := grouped[key]
		avg := 0
		if item.total > 0 {
			avg = int(item.sum / int64(item.total))
		}
		out.points = append(out.points, TrendPoint{Time: item.at, Avg: avg})
	}
	return out, nil
}

func isProblem(m model.Monitor) bool {
	if m.Status == model.StatusDown {
		return true
	}
	switch m.TLSStatus {
	case model.TLSWarning, model.TLSCritical, model.TLSExpired, model.TLSInvalid:
		return true
	default:
		return false
	}
}

func StatusOrder() string {
	return statusOrder
}

const statusOrder = `CASE
  WHEN status = 'DOWN' THEN 0
  WHEN tls_status IN ('EXPIRED','INVALID','CRITICAL','WARNING') THEN 1
  WHEN status = 'UNKNOWN' THEN 2
  WHEN status = 'UP' THEN 3
  WHEN status = 'PAUSED' THEN 4
  ELSE 5 END,
CASE tls_status
  WHEN 'EXPIRED' THEN 0
  WHEN 'INVALID' THEN 1
  WHEN 'CRITICAL' THEN 2
  WHEN 'WARNING' THEN 3
  ELSE 4 END,
id DESC`
