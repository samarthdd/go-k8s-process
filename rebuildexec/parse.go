package rebuildexec

import (
	"fmt"
	"log"
	"strings"
)

func parseCode(s string) string {

	str := "Glasswall process exit status = "
	if len(s) < len(str) {
		return ""
	}
	d := s[:len(str)]
	log.Println(d)
	if s[:len(str)] != str {
		return ""
	}

	s = s[len(str):]

	var statusDesc string

	for _, c := range s {

		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			statusDesc = fmt.Sprintf("%s%s", statusDesc, string(c))
		}
	}
	return statusDesc
}

func parseLogExpir(s string) string {
	str := "Zero day licence has expired"
	if len(s) < len(str) {
		return ""
	}
	offset := len(s) - len(str)
	s = s[offset:]
	if s == str {
		return "SDK EXPIRED"
	}
	return ""
}

func parseVersion(b string) string {
	sl := strings.Split(string(b), "\n")

	if len(sl) > 0 {
		return sl[0]
	}
	return ""
}

func parseContnetType(s string) string {
	sl := strings.Split(s, "/")
	if len(sl) > 1 {
		return sl[1]
	}
	return s
}
