package stable

import (
	"sync"
	"testing"
)

func TestDetector_Threshold(t *testing.T) {
	d := &Detector{
		threshold:    3,
		stableHashes: make(map[string]bool),
	}

	// Manually set a hash as stable for testing
	d.mu.Lock()
	d.stableHashes["abc123"] = true
	d.mu.Unlock()

	if !d.IsStable("abc123") {
		t.Error("Expected hash to be stable")
	}

	if d.IsStable("nonexistent") {
		t.Error("Expected nonexistent hash to not be stable")
	}
}

func TestDetector_ConcurrentAccess(t *testing.T) {
	d := &Detector{
		threshold:    5,
		stableHashes: make(map[string]bool),
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			hash := string(rune('a' + id))
			// Use the public API which handles locking internally
			// We'll just test concurrent reads
			_ = d.IsStable(hash)
		}(i)
	}

	wg.Wait()
}

func TestDetector_ConcurrentReadWrite(t *testing.T) {
	d := &Detector{
		threshold:    5,
		stableHashes: make(map[string]bool),
	}

	var wg sync.WaitGroup
	
	// Writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			hash := string(rune('a' + id))
			d.mu.Lock()
			d.stableHashes[hash] = true
			d.mu.Unlock()
		}(i)
	}
	
	// Readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			hash := string(rune('a' + id))
			_ = d.IsStable(hash)
		}(i)
	}

	wg.Wait()
	
	// Verify all writes completed
	d.mu.RLock()
	count := len(d.stableHashes)
	d.mu.RUnlock()
	
	if count != 5 {
		t.Errorf("Expected 5 stable hashes, got %d", count)
	}
}
