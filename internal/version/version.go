package version

import "fmt"

var (
	BuildDate    = "unknown"
	BuildVersion = "0.0.0"
	Commit       = "unknown"
)

func BaseVersionInfo() string {
	return fmt.Sprintf("%s (%s) on %s", BuildVersion, Commit, BuildDate)
}
