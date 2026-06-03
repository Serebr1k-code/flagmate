package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	ID        int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"not null"`
	Port      int       `json:"port" gorm:"not null"`
	Protocol  string    `json:"protocol" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

type Pattern struct {
	ID          int       `json:"id" gorm:"primaryKey;autoIncrement"`
	Pattern     string    `json:"pattern" gorm:"not null"`
	Description string    `json:"description"`
	Mode        string    `json:"mode" gorm:"default:B"`
	Active      bool      `json:"active" gorm:"default:true"`
	MatchCount  int       `json:"match_count" gorm:"default:0"`
	CreatedAt   time.Time `json:"created_at"`
}

type Flow struct {
	ID           uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey"`
	ServiceID    *int            `json:"service_id"`
	Service      *Service        `json:"service,omitempty" gorm:"foreignKey:ServiceID"`
	Direction    string          `json:"direction"`
	StartTs      *time.Time      `json:"start_ts"`
	EndTs        *time.Time      `json:"end_ts"`
	RawRequest   JSONMap         `json:"raw_request" gorm:"type:jsonb"`
	RawResponse  JSONMap         `json:"raw_response" gorm:"type:jsonb"`
	NormPayload  string          `json:"-"`
	Hash         string          `json:"hash"`
	Stable       bool            `json:"stable"`
	Checker      bool            `json:"checker"`
	Banned       bool            `json:"banned"`
	ResponseCode int             `json:"response_code"`
	FlowID       uint64          `json:"flow_id"`          // Suricata flow_id
	SrcIP        string          `json:"src_ip"`
	DstIP        string          `json:"dst_ip"`
	SrcPort      int             `json:"src_port"`
	DstPort      int              `json:"dst_port"`
	Proto        string          `json:"proto"`
	PktCount     int             `json:"pkt_count"`
	BytesIn      int64           `json:"bytes_in"`
	BytesOut     int64           `json:"bytes_out"`
	CreatedAt    time.Time       `json:"created_at"`
}

type JSONMap map[string]interface{}

func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

type FlowGroup struct {
	Hash          string    `json:"hash"`
	Count         int       `json:"count"`
	ExampleFlowID uuid.UUID `json:"example_flow_id"`
	FirstSeen     time.Time `json:"first_seen"`
	LastSeen      time.Time `json:"last_seen"`
}

type MirroringConfig struct {
	Enabled bool           `json:"enabled"`
	Targets []MirrorTarget `json:"targets"`
}

type MirrorTarget struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type LabelRequest struct {
	Checker bool `json:"checker"`
}
