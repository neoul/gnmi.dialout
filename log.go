package dialout

import "log"

// logging facilities for gnmi dialout
// - Assign custom Printf and Print functions for logging redirection.

var (
	Printf func(format string, v ...interface{}) = printf
	Print  func(v ...interface{})                = print
)

// printf calls Output to print to the standard logger.
// Arguments are handled in the manner of fmt.printf.
func printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func print(v ...interface{}) {
	log.Print(v...)
}
