package pcf

import (
	"testing"
)

func TestPcf(t *testing.T) {
	Debug = true
	if f, err := Open("wenquanyi_13px.pcf"); err == nil {
		f.DumpAscii("out", '操')
	}
}
