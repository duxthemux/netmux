// Package buildinfo allow playing with software version.
package buildinfo

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed build-date
var Date string

//go:embed build-semver
var SemVer string

//go:embed build-hash
var Hash string

// String will return a formated string with all info above.
func String(msg string) string {
	return fmt.Sprintf(`%s
Build Date:%s
SemVer: %s
Hash: %s`, msg,
		strings.TrimSpace(Date),
		strings.TrimSpace(SemVer),
		strings.TrimSpace(Hash))
}

// StringOneLine will do similar to String, but all in one line.
func StringOneLine(msg string) string {
	return fmt.Sprintf(`%s
Build Date:%s | SemVer: %s | Hash: %s`, msg,
		strings.TrimSpace(Date),
		strings.TrimSpace(SemVer),
		strings.TrimSpace(Hash))
}
