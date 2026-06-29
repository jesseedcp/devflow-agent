package version

import (
	"fmt"
	"runtime"
	"strings"
)

var Version = "dev"
var Commit = "unknown"
var Date = "unknown"

type Info struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	OS        string
	Arch      string
}

func Current() Info {
	return Info{
		Version:   clean(Version),
		Commit:    clean(Commit),
		Date:      clean(Date),
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

func (i Info) String() string {
	lines := []string{
		fmt.Sprintf("version: %s", clean(i.Version)),
		fmt.Sprintf("commit: %s", clean(i.Commit)),
		fmt.Sprintf("date: %s", clean(i.Date)),
		fmt.Sprintf("go: %s", clean(i.GoVersion)),
		fmt.Sprintf("platform: %s/%s", clean(i.OS), clean(i.Arch)),
	}
	return strings.Join(lines, "\n")
}

func clean(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return strings.Join(strings.Fields(value), " ")
}
