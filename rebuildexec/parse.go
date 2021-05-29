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
		return RebuildStatusExpired
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

func parseStatus(b string) string {

	if len(b) > 200 {

		b = (b[(len(b) - 200):])

	}

	sl := strings.Split(string(b), "\n")
	for _, s := range sl {
		statusdesc := parseCode(s)
		if statusdesc != "" {
			return statusdesc
		}
		statusdesc = parseLogExpir(s)
		if statusdesc != "" {
			return statusdesc
		}

	}

	return RebuildStatusUnprocessable

}
