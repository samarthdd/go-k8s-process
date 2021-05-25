package rebuildexec

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestZipProcess(t *testing.T) {

	zipfileCopy()
	zipPath := filepath.Join(mainProjectPath, "/tmp/zip/input")

	zipProc := zipProcess{
		workdir:   zipPath,
		zipEntity: nil,
		ext:       "",
	}

	err := zipProc.openZip("nested.zip")
	if err != nil {
		t.Error(err)
	}
	zipProc.readAllFilesExt("")
	zipProc.workdir = filepath.Join(mainProjectPath, "/tmp/zip/output")
	zipProc.writeZip("nested.zip")
	if err != nil {
		t.Error(err)
	}

	t.Cleanup(func() {
		os.RemoveAll(filepath.Join(mainProjectPath, "/tmp/zip"))
	})

}

func zipfileCopy() error {

	os.MkdirAll(filepath.Join(mainProjectPath, "/tmp/zip/input"), 0777)
	os.MkdirAll(filepath.Join(mainProjectPath, "/tmp/zip/output"), 0777)
	absouluteZip := filepath.Join(mainProjectPath, depDir, "nested.zip")

	cmd := exec.Command("cp", absouluteZip, filepath.Join(mainProjectPath, "/tmp/zip/input", "nested.zip"))
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
