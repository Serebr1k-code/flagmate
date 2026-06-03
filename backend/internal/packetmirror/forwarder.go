package packetmirror

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/afpacket"
	"github.com/google/gopacket/layers"
)

type PacketForwarder struct {
	mu          sync.RWMutex
	targets     []ForwardTarget
	handle      *afpacket.TPacket
	bufferSize  int
	frameSize   int
	numFrames   int
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
}

type ForwardTarget struct {
	IP   string
	Port int
	Conn net.Conn
}

func NewPacketForwarder(bufferSize int, frameSize int, numFrames int) *PacketForwarder {
	return &PacketForwarder{
		bufferSize: bufferSize,
		frameSize:  frameSize,
		numFrames:  numFrames,
		targets:    make([]ForwardTarget, 0),
	}
}

func (pf *PacketForwarder) Start(queueNum int) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if pf.running {
		return nil
	}

	handle, err := afpacket.NewTPacket(
		afpacket.OptFrameSize(pf.frameSize),
		afpacket.OptBlockSize(pf.bufferSize),
		afpacket.OptNumBlocks(pf.numFrames),
	)
	if err != nil {
		return logError("failed to create AF_PACKET handle: %v", err)
	}

	pf.handle = handle
	pf.running = true
	pf.ctx, pf.cancel = context.WithCancel(context.Background())

	go pf.readLoop()
	log.Println("Packet forwarder started")
	return nil
}

func (pf *PacketForwarder) Stop() {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	if !pf.running {
		return
	}

	pf.cancel()
	if pf.handle != nil {
		pf.handle.Close()
	}
	pf.running = false

	for i := range pf.targets {
		if pf.targets[i].Conn != nil {
			pf.targets[i].Conn.Close()
		}
	}

	log.Println("Packet forwarder stopped")
}

func (pf *PacketForwarder) AddTarget(ip string, port int) error {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return logError("failed to connect to %s: %v", addr, err)
	}

	pf.targets = append(pf.targets, ForwardTarget{
		IP:   ip,
		Port: port,
		Conn: conn,
	})

	log.Printf("Packet forward target added: %s", addr)
	return nil
}

func (pf *PacketForwarder) RemoveTarget(ip string, port int) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	var newTargets []ForwardTarget

	for _, t := range pf.targets {
		if t.IP == ip && t.Port == port {
			if t.Conn != nil {
				t.Conn.Close()
			}
			log.Printf("Packet forward target removed: %s", addr)
		} else {
			newTargets = append(newTargets, t)
		}
	}

	pf.targets = newTargets
}

func (pf *PacketForwarder) readLoop() {
	for {
		select {
		case <-pf.ctx.Done():
			return
		default:
		}

		data, ci, err := pf.handle.ReadPacketData()
		if err != nil {
			log.Printf("Error reading packet: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		pf.forwardPacket(data, ci)
	}
}

func (pf *PacketForwarder) forwardPacket(data []byte, ci gopacket.CaptureInfo) {
	pf.mu.RLock()
	targets := make([]ForwardTarget, len(pf.targets))
	copy(targets, pf.targets)
	pf.mu.RUnlock()

	if len(targets) == 0 {
		return
	}

	packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.NoCopy)
	if packet == nil {
		return
	}

	for i := range targets {
		if targets[i].Conn == nil {
			continue
		}

		go func(t ForwardTarget) {
			t.Conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if _, err := t.Conn.Write(data); err != nil {
				log.Printf("Failed to forward packet to %s:%d: %v", t.IP, t.Port, err)
				t.Conn.Close()
			}
		}(targets[i])
	}
}

func logError(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	log.Printf("PacketMirror: %v", err)
	return err
}
