package rebuildexec

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	validVersionOutput := `1.221
SUCCESS
`
	nonValidVersionOutput := "error no such command"
	emptyOutput := ""

	versionTest := []struct {
		text    string
		version string
	}{
		{validVersionOutput, "1.221"},
		{nonValidVersionOutput, "error no such command"},
		{emptyOutput, ""},
	}
	for _, v := range versionTest {
		res := parseVersion(v.text)
		if res != v.version {
			if v.version == "" {

				t.Errorf("fails expected empty string got %s", res)
			} else {
				t.Errorf("fails expected %s got %s", v.version, res)

			}

		}
	}
}
