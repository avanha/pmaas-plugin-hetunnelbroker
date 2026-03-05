package data

import "time"

type TunnelData struct {
	LastUpdateTime     time.Time
	LastModifiedTime   time.Time
	GetSuccessCount    int
	GetErrorCount      int
	UpdateSuccessCount int
	UpdateErrorCount   int
	LastError          error
	LastErrorTime      time.Time
}
