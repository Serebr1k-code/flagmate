package mirror

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Target struct {
	IP   string
	Port int
}

type Mirror struct {
	targets []Target
	mu      sync.RWMutex
	enabled bool
	conns   []net.Conn
}

func New() *Mirror {
	return &Mirror{
		conns: make([]net.Conn, 0),
	}
}

func (m *Mirror) SetEnabled(enabled bool) {
	m.mu.Lock()
	m.enabled = enabled
	m.mu.Unlock()

	if !enabled {
		m.closeAll()
	}
}

func (m *Mirror) SetTargets(targets []Target) {
	m.mu.Lock()
	m.targets = targets
	m.mu.Unlock()

	m.reconnectAll()
}

func (m *Mirror) Broadcast(line string) {
	m.mu.RLock()
	enabled := m.enabled
	targets := make([]Target, len(m.targets))
	copy(targets, m.targets)
	conns := make([]net.Conn, len(m.conns))
	copy(conns, m.conns)
	m.mu.RUnlock()

	if !enabled {
		return
	}

	for i, conn := range conns {
		if conn == nil {
			continue
		}
		go func(idx int, c net.Conn) {
			if c == nil {
				return
			}
			c.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if _, err := fmt.Fprintln(c, line); err != nil {
				m.reconnect(idx)
			}
		}(i, conn)
	}
}

func (m *Mirror) reconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closeAll()
	m.conns = make([]net.Conn, len(m.targets))

	for i, t := range m.targets {
		go m.connectAndStore(i, t)
	}
}

func (m *Mirror) connectAndStore(idx int, t Target) {
	addr := fmt.Sprintf("%s:%d", t.IP, t.Port)
	for {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			m.mu.Lock()
			m.conns[idx] = conn
			m.mu.Unlock()
			return
		}
		time.Sleep(3 * time.Second)
	}
}

func (m *Mirror) reconnect(idx int) {
	m.mu.Lock()
	if m.conns[idx] != nil {
		m.conns[idx].Close()
		m.conns[idx] = nil
	}
	target := m.targets[idx]
	m.mu.Unlock()

	go m.connectAndStore(idx, target)
}

func (m *Mirror) closeAll() {
	for _, conn := range m.conns {
		if conn != nil {
			conn.Close()
		}
	}
	m.conns = make([]net.Conn, 0)
}

func (m *Mirror) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}
