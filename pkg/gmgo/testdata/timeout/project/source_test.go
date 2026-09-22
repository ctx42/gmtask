package project

import (
	"testing"
	"time"
)

// Test_Hello blocks so that a "go test -timeout" short enough to expire can
// never lose the race to a test that has already finished. The sleep is never
// waited out: the timeout fires first and the binary is killed, so the length
// only has to outlast the scheduling delay before the watchdog runs.
func Test_Hello(t *testing.T) {
	if Hello() != "hello world" {
		t.Error("expected different result")
	}
	time.Sleep(5 * time.Second)
}
