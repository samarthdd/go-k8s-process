package rebuildexec

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"testing"
)

const (
	cmpJsonSample = `{
        "PdfContentManagement": {
            "Metadata": 1,
            "InternalHyperlinks": 1,
            "ExternalHyperlinks": 1,
            "EmbeddedFiles": 1,
            "EmbeddedImages": 1,
            "Javascript": 1,
            "Acroform": 1,
            "ActionsAll": 1
        },
        "ExcelContentManagement": {
            "Metadata": 2,
            "InternalHyperlinks": 2,
            "ExternalHyperlinks": 2,
            "EmbeddedFiles": 1,
            "EmbeddedImages": 0,
            "DynamicDataExchange": 0,
            "Macros": 0,
            "ReviewComments": 0
        },
        "PowerPointContentManagement": {
            "Metadata": 0,
            "InternalHyperlinks": 0,
            "ExternalHyperlinks": 0,
            "EmbeddedFiles": 0,
            "EmbeddedImages": 0,
            "Macros": 0,
            "ReviewComments": 0
            
        },
        "WordContentManagement": {
            "Metadata": 0,
            "InternalHyperlinks": 0,
            "ExternalHyperlinks": 0,
            "EmbeddedFiles": 0,
            "EmbeddedImages": 0,
            "DynamicDataExchange": 0,
            "Macros": 0,
            "ReviewComments": 0
        }
}`
)

func TestCmp(t *testing.T) {

	bjson := []byte(cmpJsonSample)

	bjson = bytes.TrimPrefix(bjson, []byte("\xef\xbb\xbf"))
	p, err := cmpJsonMarshal(bjson)
	if err != nil {
		t.Error(err)
	}
	b, err := p.cmpXmlconv()
	if err != nil {
		t.Error(err)
	}
	s := filepath.Join(mainProjectPath, "tmp", "xml.xml")
	ioutil.WriteFile(s, b, 0777)

}

//get
