package project

import (
	"testing"
)

func Test_Hello(t *testing.T) {
	if Hello() != "hello world" {
		t.Error("expected different result")
	}
}
