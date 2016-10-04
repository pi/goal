//+build !debug

package debug

const Enabled = false

func Log(format string, args ...interface{}) {}
