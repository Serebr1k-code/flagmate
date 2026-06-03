package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/flagmate/suricata-ctf/backend/internal/eve"
	"github.com/flagmate/suricata-ctf/backend/internal/models"
	"github.com/flagmate/suricata-ctf/backend/internal/normaliser"
	"github.com/google/uuid"
)

type Assembler struct {
	activeFlows map[uint64]*FlowContext
	mu          sync.RWMutex
	normaliser  *normaliser.Normaliser
}

type FlowContext struct {
	FlowID      uint64
	SrcIP       string
	SrcPort     int
	DstIP       string
	DstPort     int
	Proto       string
	StartTs     *time.Time
	EndTs       *time.Time
	Request     *models.JSONMap
	Response    *models.JSONMap
	NormPayload string
	Hash        string
	PktCount    int
	BytesIn     int64
	BytesOut    int64
	ResponseCode int
	ServiceID   *int
}

func NewAssembler(n *normaliser.Normaliser) *Assembler {
	a := &Assembler{
		activeFlows: make(map[uint64]*FlowContext),
		normaliser:  n,
	}
	go a.cleanupLoop()
	return a
}

func (a *Assembler) ProcessEvent(event eve.Event) *FlowContext {
	a.mu.Lock()
	defer a.mu.Unlock()

	ctx, exists := a.activeFlows[event.FlowID]
	if !exists {
		ctx = &FlowContext{
			FlowID:  event.FlowID,
			SrcIP:   event.SrcIP,
			SrcPort: event.SrcPort,
			DstIP:   event.DstIP,
			DstPort: event.DstPort,
			Proto:   event.Proto,
		}
		if event.Flow != nil {
			if t, err := time.Parse(time.RFC3339, event.Flow.Start); err == nil {
				ctx.StartTs = &t
			}
			ctx.PktCount = event.Flow.PktsToserver + event.Flow.PktsToclient
			ctx.BytesIn = event.Flow.BytesToclient
			ctx.BytesOut = event.Flow.BytesToserver
		}
		a.activeFlows[event.FlowID] = ctx
	}

	if event.HTTP != nil {
		if event.HTTP.HTTPRequest != nil {
			req := models.JSONMap{
				"method":   event.HTTP.HTTPRequest.Method,
				"uri":      event.HTTP.HTTPRequest.URI,
				"protocol": event.HTTP.HTTPRequest.Protocol,
				"headers":  event.HTTP.HTTPRequest.Headers,
				"body":     event.HTTP.HTTPRequest.Body,
			}
			ctx.Request = &req
		}
		if event.HTTP.HTTPResponse != nil {
			resp := models.JSONMap{
				"status":   event.HTTP.HTTPResponse.Status,
				"protocol": event.HTTP.HTTPResponse.Protocol,
				"headers":  event.HTTP.HTTPResponse.Headers,
				"body":     event.HTTP.HTTPResponse.Body,
			}
			ctx.Response = &resp
			ctx.ResponseCode = event.HTTP.HTTPResponse.Status
		}
	}

	if event.Flow != nil && event.Flow.End != "" {
		if t, err := time.Parse(time.RFC3339, event.Flow.End); err == nil {
			ctx.EndTs = &t
		}
		ctx.PktCount = event.Flow.PktsToserver + event.Flow.PktsToclient
		ctx.BytesIn = event.Flow.BytesToclient
		ctx.BytesOut = event.Flow.BytesToserver
	}

	if ctx.Request != nil && ctx.Response != nil && ctx.Hash == "" {
		a.finaliseFlow(ctx)
		delete(a.activeFlows, event.FlowID)
		return ctx
	}

	return nil
}

func (a *Assembler) finaliseFlow(ctx *FlowContext) {
	payload := ""
	if ctx.Request != nil {
		payload += fmt.Sprintf("%+v", ctx.Request)
	}
	if ctx.Response != nil {
		payload += fmt.Sprintf("%+v", ctx.Response)
	}

	ctx.NormPayload = a.normaliser.Normalise(payload)
	hash := sha256.Sum256([]byte(ctx.NormPayload))
	ctx.Hash = hex.EncodeToString(hash[:])
}

func (a *Assembler) ToModel(ctx *FlowContext) models.Flow {
	direction := "unknown"
	if ctx.SrcIP != "" && ctx.DstIP != "" {
		direction = fmt.Sprintf("%s:%d → %s:%d", ctx.SrcIP, ctx.SrcPort, ctx.DstIP, ctx.DstPort)
	}

	flow := models.Flow{
		ID:           uuid.New(),
		FlowID:       ctx.FlowID,
		ServiceID:    ctx.ServiceID,
		Direction:    direction,
		StartTs:      ctx.StartTs,
		EndTs:        ctx.EndTs,
		RawRequest:   *ctx.Request,
		RawResponse:  *ctx.Response,
		NormPayload:  ctx.NormPayload,
		Hash:         ctx.Hash,
		ResponseCode: ctx.ResponseCode,
		SrcIP:        ctx.SrcIP,
		DstIP:        ctx.DstIP,
		SrcPort:      ctx.SrcPort,
		DstPort:      ctx.DstPort,
		Proto:        ctx.Proto,
		PktCount:     ctx.PktCount,
		BytesIn:      ctx.BytesIn,
		BytesOut:     ctx.BytesOut,
		CreatedAt:    time.Now(),
	}
	return flow
}

func (a *Assembler) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		a.mu.Lock()
		now := time.Now()
		for id, ctx := range a.activeFlows {
			if ctx.StartTs != nil && now.Sub(*ctx.StartTs) > 5*time.Minute {
				delete(a.activeFlows, id)
			}
		}
		a.mu.Unlock()
	}
}
