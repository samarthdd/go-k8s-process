package rebuildexec

import "testing"

func TestOpenZip(t *testing.T) {
	zipProc := zipProcess{
		workdir:   "/home/ibrahim/Desktop/gwsample/test",
		zipEntity: nil,
		ext:       "",
	}
	err := zipProc.openZip("blown.zip")
	if err != nil {
		t.Error(err)
	}
	zipProc.readAllFilesExt("")

	zipProc.writeZip("blown2.zip")
	if err != nil {
		t.Error(err)
	}

}
