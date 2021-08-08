package filelog

import (
	"fmt"
	"testing"
)

func TestNewFileHook(t *testing.T) {
	f, _ := NewFileHook(&Config{})
	count := 1
	for {
		_ = f.Write([]byte(fmt.Sprintf("%v", count)))
		count += 1
	}
}
