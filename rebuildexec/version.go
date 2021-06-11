package rebuildexec

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

var once sync.Once
var rebuildSdkVersion string

func GetSdkVersion() string {
	once.Do(func() {
		rebuildSdkVersion = GetVersion()
	})

	return rebuildSdkVersion
}

func GetVersion() string {

	app := os.Getenv("GWCLI")
	args := fmt.Sprintf("%s -v", app)

	b, err := gwCliExec(args)
	if err != nil {
		b = []byte(err.Error())
	}

	s := parseVersion(string(b))

	return s
}
func parseVersion(b string) string {
	sl := strings.Split(string(b), "\n")

	if len(sl) > 0 {
		return sl[0]
	}
	return ""
}
