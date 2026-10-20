package project

import (
	"testing"
	"time"
)

// Test_Hello prints nothing for far longer than the stall the watchdog test
// sets, so the watchdog signals the test binary before the sleep ends.
func Test_Hello(t *testing.T) {
	if Hello() != "hello world" {
		t.Error("expected different result")
	}
	time.Sleep(10 * time.Minute)
}
