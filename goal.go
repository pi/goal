// dummy package to go build all subpackages
package goal

import (
    _ "github.com/pi/goal/bits"
    _ "github.com/pi/goal/gut"
    _ "github.com/pi/goal/hash"
    _ "github.com/pi/goal/md"
    _ "github.com/pi/goal/th"
    _ "github.com/pi/goal/pipe"
    _ "github.com/pi/goal/ringbuf"
)
