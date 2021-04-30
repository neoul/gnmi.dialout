package dialout

// logging facilities for gnmi dialout
// - Assign custom LogPrintf and Print functions for logging redirection.

var (
	Printf func(format string, v ...interface{})
	Print  func(v ...interface{})
)

// printf calls Output to print to the standard logger.
// Arguments are handled in the manner of fmt.printf.
func LogPrintf(format string, v ...interface{}) {
	if Printf != nil {
		Printf(format, v...)
	}
}

func LogPrint(v ...interface{}) {
	if Print != nil {
		Print(v...)
	}
}
