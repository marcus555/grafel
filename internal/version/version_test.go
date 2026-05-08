package version

import (
	"strings"
	"testing"
)

func TestStringContainsAllParts(t *testing.T) {
	s := String()
	if !strings.Contains(s, "commit") {
		t.Errorf("version string missing 'commit': %s", s)
	}
	if !strings.Contains(s, "built") {
		t.Errorf("version string missing 'built': %s", s)
	}
}

func TestConcurrentStringNoRace(t *testing.T) {
	// Run String() concurrently — `go test -race` will flag any
	// package-state mutation.
	done := make(chan struct{}, 16)
	for i := 0; i < 16; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = String()
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 16; i++ {
		<-done
	}
}
