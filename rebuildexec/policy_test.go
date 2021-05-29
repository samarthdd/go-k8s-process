package rebuildexec

import (
	"io/ioutil"
	"log"
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
	b := cmpJsontoXml([]byte(cmpJsonSample))
	ioutil.WriteFile("xml.xml", b, 0777)
	log.Println(string(b))
}

//get
