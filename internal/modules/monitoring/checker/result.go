package checker

import "time"

type Result struct {
	Success      bool           `json:"success"`
	Status       string         `json:"status"`
	StatusCode   int            `json:"statusCode"`
	ResponseTime int            `json:"responseTime"`
	Message      string         `json:"message"`
	CheckedAt    time.Time      `json:"checkedAt"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	// Immediate bypasses the failure threshold. Used for stable certificate failures.
	Immediate bool     `json:"immediate"`
	TLS       *TLSInfo `json:"tls,omitempty"`
}

type TLSInfo struct {
	CN            string    `json:"cn"`
	SAN           string    `json:"san"`
	Issuer        string    `json:"issuer"`
	Serial        string    `json:"serial"`
	NotBefore     time.Time `json:"notBefore"`
	NotAfter      time.Time `json:"notAfter"`
	DaysRemaining int       `json:"daysRemaining"`
	Status        string    `json:"status"`
}
