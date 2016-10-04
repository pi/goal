//+build debug

package debug

import (
	"fmt"
	"log"
)

const Enabled = true

func Log(format string, args ...interface{}) {
	log.Println("[DEBUG]" + fmt.Sprintf(format, args...))
}
