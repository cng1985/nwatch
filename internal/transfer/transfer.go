package transfer

import (
	"time"

	"github.com/cng1985/nwatch/internal/model"
	"gorm.io/gorm"
)

type Bundle struct {
	Version    int            `json:"version"`
	ExportedAt time.Time      `json:"exportedAt"`
	Groups     []GroupItem    `json:"groups"`
	Notifiers  []NotifierItem `json:"notifiers"`
	Monitors   []MonitorItem  `json:"monitors"`
}

type GroupItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
}

type NotifierItem struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	Secret     string `json:"secret,omitempty"`
	Enabled    bool   `json:"enabled"`
}

type MonitorItem struct {
	Name                string            `json:"name"`
	Group               string            `json:"group,omitempty"`
	Type                string            `json:"type"`
	URL                 string            `json:"url,omitempty"`
	Host                string            `json:"host,omitempty"`
	Port                int               `json:"port,omitempty"`
	Method              string            `json:"method,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	Body                string            `json:"body,omitempty"`
	Interval            int               `json:"interval"`
	Timeout             int               `json:"timeout"`
	Enabled             bool              `json:"enabled"`
	FailureThreshold    int               `json:"failureThreshold"`
	RecoveryThreshold   int               `json:"recoveryThreshold"`
	ExpectedStatusCodes string            `json:"expectedStatusCodes,omitempty"`
	BodyContains        string            `json:"bodyContains,omitempty"`
	BodyNotContains     string            `json:"bodyNotContains,omitempty"`
	FollowRedirects     bool              `json:"followRedirects"`
	TLSWarningDays      int               `json:"tlsWarningDays,omitempty"`
	TLSCriticalDays     int               `json:"tlsCriticalDays,omitempty"`
	TLSNotifyDays       string            `json:"tlsNotifyDays,omitempty"`
	Notifiers           []string          `json:"notifiers,omitempty"`
}

func Export(db *gorm.DB, includeSecrets bool) (Bundle, error) {
	var groups []model.Group
	var notifiers []model.Notifier
	var monitors []model.Monitor
	if err := db.Order("sort, id").Find(&groups).Error; err != nil {
		return Bundle{}, err
	}
	if err := db.Order("id").Find(&notifiers).Error; err != nil {
		return Bundle{}, err
	}
	if err := db.Preload("Group").Preload("Notifiers").Order("id").Find(&monitors).Error; err != nil {
		return Bundle{}, err
	}
	out := Bundle{Version: 1, ExportedAt: time.Now()}
	for _, g := range groups {
		out.Groups = append(out.Groups, GroupItem{Name: g.Name, Description: g.Description, Sort: g.Sort})
	}
	for _, n := range notifiers {
		item := NotifierItem{Name: n.Name, Type: n.Type, Enabled: n.Enabled}
		if includeSecrets {
			item.WebhookURL = n.WebhookURL
			item.Secret = n.Secret
		}
		out.Notifiers = append(out.Notifiers, item)
	}
	for _, m := range monitors {
		item := MonitorItem{
			Name: m.Name, Type: m.Type, URL: m.URL, Host: m.Host, Port: m.Port, Method: m.Method,
			Headers: m.Headers, Body: m.Body, Interval: m.Interval, Timeout: m.Timeout, Enabled: m.Enabled,
			FailureThreshold: m.FailureThreshold, RecoveryThreshold: m.RecoveryThreshold,
			ExpectedStatusCodes: m.ExpectedStatusCodes, BodyContains: m.BodyContains, BodyNotContains: m.BodyNotContains,
			FollowRedirects: m.FollowRedirects, TLSWarningDays: m.TLSWarningDays, TLSCriticalDays: m.TLSCriticalDays,
			TLSNotifyDays: m.TLSNotifyDays,
		}
		if m.Group != nil {
			item.Group = m.Group.Name
		}
		for _, n := range m.Notifiers {
			item.Notifiers = append(item.Notifiers, n.Name)
		}
		out.Monitors = append(out.Monitors, item)
	}
	if out.Groups == nil {
		out.Groups = []GroupItem{}
	}
	if out.Notifiers == nil {
		out.Notifiers = []NotifierItem{}
	}
	if out.Monitors == nil {
		out.Monitors = []MonitorItem{}
	}
	return out, nil
}

func Import(db *gorm.DB, b Bundle) error {
	return db.Transaction(func(tx *gorm.DB) error {
		groupIDs := map[string]uint{}
		for _, g := range b.Groups {
			if g.Name == "" {
				continue
			}
			var row model.Group
			err := tx.Where("name = ?", g.Name).First(&row).Error
			if err == gorm.ErrRecordNotFound {
				row = model.Group{Name: g.Name, Description: g.Description, Sort: g.Sort}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				row.Description = g.Description
				row.Sort = g.Sort
				if err := tx.Save(&row).Error; err != nil {
					return err
				}
			}
			groupIDs[g.Name] = row.ID
		}
		notifierIDs := map[string]uint{}
		for _, n := range b.Notifiers {
			if n.Name == "" {
				continue
			}
			var row model.Notifier
			err := tx.Where("name = ?", n.Name).First(&row).Error
			if err == gorm.ErrRecordNotFound {
				row = model.Notifier{Name: n.Name, Type: n.Type, WebhookURL: n.WebhookURL, Secret: n.Secret, Enabled: n.Enabled}
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				row.Type = n.Type
				row.Enabled = n.Enabled
				if n.WebhookURL != "" {
					row.WebhookURL = n.WebhookURL
				}
				if n.Secret != "" {
					row.Secret = n.Secret
				}
				if err := tx.Save(&row).Error; err != nil {
					return err
				}
			}
			notifierIDs[n.Name] = row.ID
		}
		for _, item := range b.Monitors {
			if item.Name == "" {
				continue
			}
			var row model.Monitor
			err := tx.Where("name = ?", item.Name).First(&row).Error
			creating := err == gorm.ErrRecordNotFound
			if err != nil && !creating {
				return err
			}
			row.Name = item.Name
			row.Type = item.Type
			row.URL = item.URL
			row.Host = item.Host
			row.Port = item.Port
			row.Method = item.Method
			row.Headers = item.Headers
			row.SyncHeaders()
			row.Body = item.Body
			row.Interval = item.Interval
			row.Timeout = item.Timeout
			row.Enabled = item.Enabled
			row.FailureThreshold = item.FailureThreshold
			row.RecoveryThreshold = item.RecoveryThreshold
			row.ExpectedStatusCodes = item.ExpectedStatusCodes
			row.BodyContains = item.BodyContains
			row.BodyNotContains = item.BodyNotContains
			row.FollowRedirects = item.FollowRedirects
			row.TLSWarningDays = item.TLSWarningDays
			row.TLSCriticalDays = item.TLSCriticalDays
			row.TLSNotifyDays = item.TLSNotifyDays
			if id, ok := groupIDs[item.Group]; ok {
				gid := id
				row.GroupID = &gid
			}
			if creating {
				now := time.Now()
				row.Status = model.StatusUnknown
				if !row.Enabled {
					row.Status = model.StatusPaused
				}
				row.NextCheckAt = &now
				if err := tx.Create(&row).Error; err != nil {
					return err
				}
			} else if err := tx.Save(&row).Error; err != nil {
				return err
			}
			var links []model.Notifier
			for _, name := range item.Notifiers {
				if id, ok := notifierIDs[name]; ok {
					links = append(links, model.Notifier{ID: id})
				}
			}
			if err := tx.Model(&row).Association("Notifiers").Replace(links); err != nil {
				return err
			}
		}
		return nil
	})
}
