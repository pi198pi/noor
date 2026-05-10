package main

import (
	"os"

	clog "github.com/charmbracelet/log"
)

// Log is the package-level structured logger. It is silent by default
// (level = ERROR) and switched to DEBUG when --debug or DEBUG=1 is set.
var Log = clog.NewWithOptions(os.Stderr, clog.Options{
	ReportTimestamp: true,
	Level:           clog.ErrorLevel,
	Prefix:          "noor",
})

// EnableDebugLogging raises the log level to DEBUG.
func EnableDebugLogging() {
	Log.SetLevel(clog.DebugLevel)
	Log.Debug("debug logging enabled")
}
