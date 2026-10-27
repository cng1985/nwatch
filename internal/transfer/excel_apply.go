package transfer

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"github.com/cng1985/nwatch/internal/monitor"
	"gorm.io/gorm"
)

// ImportStats 是一次导入新建和更新的数量。
type ImportStats struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

type plan struct {
	row       SheetMonitor
	req       monitor.Request
	existing  *model.Monitor
	notifiers []model.Notifier
}

// ImportMonitorsXLSX 读取表格并写入监控。校验失败时不会改动已有数据。
// 编号存在则按编号更新；编号为空时按名称更新唯一同名监控，否则新建。
func ImportMonitorsXLSX(db *gorm.DB, r io.Reader, defaultInterval, defaultTimeout int) (ImportStats, error) {
	rows, err := ParseMonitorXLSX(r)
	if err != nil {
		return ImportStats{}, err
	}
	return ApplyMonitorSheet(db, rows, defaultInterval, defaultTimeout)
}

// ApplyMonitorSheet 按已经解析好的行写入监控。
func ApplyMonitorSheet(db *gorm.DB, rows []SheetMonitor, defaultInterval, defaultTimeout int) (ImportStats, error) {
	if len(rows) > maxImport {
		return ImportStats{}, sheetErr(fmt.Sprintf("一次最多导入 %d 条监控", maxImport))
	}
	var stats ImportStats
	err := db.Transaction(func(tx *gorm.DB) error {
		plans := make([]plan, 0, len(rows))
		var rowErrs []string
		for _, row := range rows {
			item, err := prepareMonitor(tx, row, defaultInterval, defaultTimeout)
			if err != nil {
				rowErrs = append(rowErrs, fmt.Sprintf("第%d行：%s", row.Line, err.Error()))
				continue
			}
			plans = append(plans, item)
		}
		if len(rowErrs) > 0 {
			return &RowErrors{Messages: rowErrs}
		}
		if err := rejectConflicts(plans); err != nil {
			return err
		}
		created, updated := 0, 0
		for _, item := range plans {
			if err := writeMonitor(tx, item); err != nil {
				return err
			}
			if item.existing == nil {
				created++
			} else {
				updated++
			}
		}
		stats = ImportStats{Created: created, Updated: updated, Total: created + updated}
		return nil
	})
	if err != nil {
		return ImportStats{}, err
	}
	return stats, nil
}

func prepareMonitor(tx *gorm.DB, row SheetMonitor, defaultInterval, defaultTimeout int) (plan, error) {
	row.Type = canonType(row.Type)
	row.Match = canonMatch(row.Match)
	row.BodyType = canonBodyType(row.BodyType)
	groupID, err := ensureGroup(tx, row.Group)
	if err != nil {
		return plan{}, err
	}
	links, err := notifiersByName(tx, row.Notifiers)
	if err != nil {
		return plan{}, err
	}
	existing, err := findExisting(tx, row)
	if err != nil {
		return plan{}, err
	}
	req := row.toRequest(groupID, notifierIDs(links))
	if existing != nil && req.Enabled == nil {
		enabled := existing.Enabled
		req.Enabled = &enabled
	}
	if existing != nil && req.FollowRedirects == nil {
		follow := existing.FollowRedirects
		req.FollowRedirects = &follow
	}
	if err := req.Normalize(defaultInterval, defaultTimeout); err != nil {
		return plan{}, err
	}
	return plan{row: row, req: req, existing: existing, notifiers: links}, nil
}

func (row SheetMonitor) toRequest(groupID *uint, ids []uint) monitor.Request {
	return monitor.Request{
		Name:                row.Name,
		GroupID:             groupID,
		Type:                row.Type,
		URL:                 row.URL,
		Host:                row.Host,
		Port:                row.Port,
		Method:              row.Method,
		Headers:             row.Headers,
		Body:                row.Body,
		BodyType:            row.BodyType,
		Threshold:           row.Threshold,
		MemThreshold:        row.MemThreshold,
		Match:               row.Match,
		Command:             row.Command,
		WorkDir:             row.WorkDir,
		Interval:            row.Interval,
		Timeout:             row.Timeout,
		FailureThreshold:    row.FailureThreshold,
		RecoveryThreshold:   row.RecoveryThreshold,
		ExpectedStatusCodes: row.ExpectedStatusCodes,
		BodyContains:        row.BodyContains,
		BodyNotContains:     row.BodyNotContains,
		FollowRedirects:     row.FollowRedirects,
		TLSWarningDays:      row.TLSWarningDays,
		TLSCriticalDays:     row.TLSCriticalDays,
		TLSNotifyDays:       row.TLSNotifyDays,
		NotifierIDs:         ids,
		Enabled:             row.Enabled,
	}
}

func ensureGroup(tx *gorm.DB, name string) (*uint, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	if len([]rune(name)) > 128 {
		return nil, errors.New("分组名称过长")
	}
	var row model.Group
	err := tx.Where("name = ?", name).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		row = model.Group{Name: name}
		if err := tx.Create(&row).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	id := row.ID
	return &id, nil
}

func notifiersByName(tx *gorm.DB, names []string) ([]model.Notifier, error) {
	if len(names) == 0 {
		return []model.Notifier{}, nil
	}
	var rows []model.Notifier
	if err := tx.Where("name IN ?", names).Find(&rows).Error; err != nil {
		return nil, err
	}
	byName := make(map[string]model.Notifier, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	out := make([]model.Notifier, 0, len(names))
	var missing []string
	for _, name := range names {
		row, ok := byName[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		out = append(out, row)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("通知渠道不存在：%s", strings.Join(missing, "、"))
	}
	return out, nil
}

func notifierIDs(items []model.Notifier) []uint {
	ids := make([]uint, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func findExisting(tx *gorm.DB, row SheetMonitor) (*model.Monitor, error) {
	if row.ID > 0 {
		var found model.Monitor
		err := tx.First(&found, row.ID).Error
		if err == nil {
			copied := found
			return &copied, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	var found []model.Monitor
	if err := tx.Where("name = ?", row.Name).Find(&found).Error; err != nil {
		return nil, err
	}
	if len(found) > 1 {
		return nil, errors.New("存在多条同名监控，请填写编号")
	}
	if len(found) == 1 {
		copied := found[0]
		return &copied, nil
	}
	return nil, nil
}

func rejectConflicts(plans []plan) error {
	var msgs []string
	seenID := map[uint]int{}
	seenName := map[string]int{}
	for _, item := range plans {
		if item.existing != nil {
			if prev, ok := seenID[item.existing.ID]; ok {
				msgs = append(msgs, fmt.Sprintf("第%d行和第%d行指向同一条监控", prev, item.row.Line))
				continue
			}
			seenID[item.existing.ID] = item.row.Line
			continue
		}
		if prev, ok := seenName[item.req.Name]; ok {
			msgs = append(msgs, fmt.Sprintf("第%d行和第%d行名称相同", prev, item.row.Line))
			continue
		}
		seenName[item.req.Name] = item.row.Line
	}
	if len(msgs) > 0 {
		return &RowErrors{Messages: msgs}
	}
	return nil
}

func writeMonitor(tx *gorm.DB, item plan) error {
	var m model.Monitor
	creating := item.existing == nil
	if !creating {
		m = *item.existing
	}
	wasEnabled := m.Enabled
	item.req.Apply(&m)
	if creating {
		monitor.TouchNew(&m)
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
	} else {
		if wasEnabled && !m.Enabled {
			m.Status = model.StatusPaused
		}
		if !wasEnabled && m.Enabled {
			m.Status = model.StatusUnknown
			m.ConsecutiveFailures = 0
			m.ConsecutiveSuccesses = 0
			now := time.Now()
			m.NextCheckAt = &now
		}
		if err := tx.Save(&m).Error; err != nil {
			return err
		}
		if m.GroupID == nil {
			if err := tx.Model(&model.Monitor{}).Where("id = ?", m.ID).Updates(map[string]any{"group_id": nil}).Error; err != nil {
				return err
			}
		}
	}
	links := item.notifiers
	if links == nil {
		links = []model.Notifier{}
	}
	return tx.Model(&m).Association("Notifiers").Replace(links)
}
