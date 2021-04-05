package rebuildexec

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestExec(t *testing.T) {
	path := "/home/ibrahim/Desktop/demopdf/水.pdf"
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Error(err)
	}
	f := New(b, "水.pdf")
	err = f.Rebuild()
	if err != nil {
		t.Error(err)
	}

	report, err := f.FileRreport()
	if err != nil {
		t.Error(err)
	}
	fil, err := f.FileProcessed()
	if err != nil {
		t.Error(err)
	}

	errf := ioutil.WriteFile("test.pdf", fil, 0777)
	if errf != nil {
		t.Error(errf)

	}

	fmt.Println(string(report))
	//f.Clean()

}
