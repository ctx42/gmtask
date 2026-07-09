package project

import (
	"testing"
)

func Test_Hello(t *testing.T) {
	if "hello world!" != Hello() {
		t.Error("expected different result")
	}
}
