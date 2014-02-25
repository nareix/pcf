package pcf

import (
	"log"
	"fmt"
	"testing"
)

func TestPcf(t *testing.T) {
	Debug = true
	if f, err := Open("wenquanyi_13px.pcf"); err == nil {
		for i, r := range "456|123/\\测试!" {
			log.Println("===", i, string(r))
			f.DumpAscii(fmt.Sprintf("out%d", i), r)
		}
	}
}
