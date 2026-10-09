package metric

import (
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

type Aggregator struct{}

func NewAggregator() *Aggregator { return &Aggregator{} }

func (a *Aggregator) Record(tx *gorm.DB, monitorID uint, at time.Time, success bool, responseTime int, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}
	local := at.In(loc)
	buckets := []struct {
		kind string
		when time.Time
	}{
		{model.BucketMinute, time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), 0, 0, loc)},
		{model.BucketHour, time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)},
		{model.BucketDay, time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)},
	}
	successN, failureN := 0, 0
	if success {
		successN = 1
	} else {
		failureN = 1
	}
	for _, b := range buckets {
		if err := upsert(tx, monitorID, b.when, b.kind, successN, failureN, responseTime); err != nil {
			return err
		}
	}
	return nil
}

func upsert(tx *gorm.DB, monitorID uint, bucket time.Time, kind string, successN, failureN, responseTime int) error {
	minV, maxV := responseTime, responseTime
	return tx.Exec(`
INSERT INTO monitor_metrics
  (monitor_id, bucket_time, bucket_type, total, success, failure, avg_response_time, max_response_time, min_response_time, sum_response_time)
VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?)
ON CONFLICT(monitor_id, bucket_time, bucket_type) DO UPDATE SET
  total = monitor_metrics.total + 1,
  success = monitor_metrics.success + excluded.success,
  failure = monitor_metrics.failure + excluded.failure,
  sum_response_time = monitor_metrics.sum_response_time + excluded.sum_response_time,
  max_response_time = CASE
    WHEN excluded.max_response_time > monitor_metrics.max_response_time THEN excluded.max_response_time
    ELSE monitor_metrics.max_response_time END,
  min_response_time = CASE
    WHEN monitor_metrics.min_response_time = 0 THEN excluded.min_response_time
    WHEN excluded.min_response_time = 0 THEN monitor_metrics.min_response_time
    WHEN excluded.min_response_time < monitor_metrics.min_response_time THEN excluded.min_response_time
    ELSE monitor_metrics.min_response_time END,
  avg_response_time = CAST((monitor_metrics.sum_response_time + excluded.sum_response_time) AS INTEGER) / (monitor_metrics.total + 1)
`, monitorID, bucket, kind, successN, failureN, responseTime, maxV, minV, responseTime).Error
}

type Summary struct {
	Total           int      `json:"total"`
	Success         int      `json:"success"`
	Failure         int      `json:"failure"`
	Availability    float64  `json:"availability"`
	AvgResponseTime int      `json:"avgResponseTime"`
	MinResponseTime int      `json:"minResponseTime"`
	MaxResponseTime int      `json:"maxResponseTime"`
	Buckets         []Bucket `json:"buckets"`
}

type Bucket struct {
	Time            time.Time `json:"time"`
	Total           int       `json:"total"`
	Success         int       `json:"success"`
	Failure         int       `json:"failure"`
	AvgResponseTime int       `json:"avgResponseTime"`
	MinResponseTime int       `json:"minResponseTime"`
	MaxResponseTime int       `json:"maxResponseTime"`
}

type Availability struct {
	Total   int
	Success int
	Failure int
	Avg     int
	Min     int
	Max     int
}

func AvailabilityByMonitor(db *gorm.DB, from time.Time, bucketType string) (map[uint]Availability, error) {
	var rows []model.MonitorMetric
	if err := db.Where("bucket_time >= ? AND bucket_type = ?", from, bucketType).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[uint]Availability{}
	sums := map[uint]int64{}
	for _, row := range rows {
		item := out[row.MonitorID]
		item.Total += row.Total
		item.Success += row.Success
		item.Failure += row.Failure
		if row.MaxResponseTime > item.Max {
			item.Max = row.MaxResponseTime
		}
		if row.MinResponseTime > 0 && (item.Min == 0 || row.MinResponseTime < item.Min) {
			item.Min = row.MinResponseTime
		}
		sums[row.MonitorID] += row.SumResponseTime
		out[row.MonitorID] = item
	}
	for id, item := range out {
		if item.Total > 0 {
			item.Avg = int(sums[id] / int64(item.Total))
			out[id] = item
		}
	}
	return out, nil
}

func Query(db *gorm.DB, monitorID uint, from time.Time, bucketType string) (Summary, error) {
	var rows []model.MonitorMetric
	q := db.Where("bucket_time >= ? AND bucket_type = ?", from, bucketType)
	if monitorID > 0 {
		q = q.Where("monitor_id = ?", monitorID)
	}
	if err := q.Order("bucket_time asc").Find(&rows).Error; err != nil {
		return Summary{}, err
	}
	var sum Summary
	var weighted int64
	minSet := false
	for _, row := range rows {
		sum.Total += row.Total
		sum.Success += row.Success
		sum.Failure += row.Failure
		weighted += row.SumResponseTime
		if row.MaxResponseTime > sum.MaxResponseTime {
			sum.MaxResponseTime = row.MaxResponseTime
		}
		if row.MinResponseTime > 0 && (!minSet || row.MinResponseTime < sum.MinResponseTime) {
			sum.MinResponseTime = row.MinResponseTime
			minSet = true
		}
		sum.Buckets = append(sum.Buckets, Bucket{
			Time:            row.BucketTime,
			Total:           row.Total,
			Success:         row.Success,
			Failure:         row.Failure,
			AvgResponseTime: row.AvgResponseTime,
			MinResponseTime: row.MinResponseTime,
			MaxResponseTime: row.MaxResponseTime,
		})
	}
	if sum.Total > 0 {
		sum.Availability = float64(sum.Success) / float64(sum.Total) * 100
		sum.AvgResponseTime = int(weighted / int64(sum.Total))
	}
	if sum.Buckets == nil {
		sum.Buckets = []Bucket{}
	}
	return sum, nil
}
