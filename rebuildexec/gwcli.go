package rebuildexec

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	APP           = "glasswallCLI"
	CONFIGINI     = "config.ini"
	XMLCONFIG     = "config.xml"
	INPUT         = "/tmp/glrebuild"
	MANAGED       = "Managed"
	NONCONFORMING = "NonConforming"
	INPUTKEY      = "inputLocation"
	OUTPUTKEY     = "outputLocation"
	SECTION       = "GWConfig"
	REBUILDINPUT  = "input"
	REBUILDOUTPUT = "output"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

var rebuildSdkVersion string

func init() {
	rand.Seed(time.Now().UnixNano())
	os.MkdirAll(INPUT, 0777)
	rebuildSdkVersion = sdkVersion()
}

func GetSdkVersion() string {
	return rebuildSdkVersion
}

type GwRebuild struct {
	File     []byte
	FileName string
	RandDir  string
	path     string

	statusMessage string
}

func New(file []byte, fileName, randDir string) GwRebuild {
	rebuilPath := filepath.Join(INPUT, randDir)
	os.MkdirAll(rebuilPath, 0777)
	inputRebuildpath := filepath.Join(rebuilPath, REBUILDINPUT)
	os.MkdirAll(inputRebuildpath, 0777)
	outputRebuildpath := filepath.Join(rebuilPath, REBUILDOUTPUT)

	os.MkdirAll(outputRebuildpath, 0777)

	gwRebuild := GwRebuild{
		File:          file,
		FileName:      fileName,
		RandDir:       randDir,
		path:          rebuilPath,
		statusMessage: "",
	}

	return gwRebuild
}

func (r *GwRebuild) Rebuild() error {

	var err error

	path := fmt.Sprintf("%s/%s/%s", r.path, REBUILDINPUT, r.FileName)

	err = ioutil.WriteFile(path, r.File, 0666)
	if err != nil {
		r.statusMessage = "INTERNAL ERROR"
		return err
	}
	err = r.exe()
	if err != nil {
		r.statusMessage = "INTERNAL ERROR"

		return err
	}

	err = r.RebuildStatus()
	if err != nil {
		r.statusMessage = "INTERNAL ERROR"

		return err
	}

	return nil
}

func (r *GwRebuild) Clean() error {
	err := os.RemoveAll(r.path)
	return err
}

func (r *GwRebuild) FileProcessed() ([]byte, error) {
	b, err := r.retrieveGwFile("")
	if err != nil {
		return nil, fmt.Errorf("processed file not found")
	}
	return b, nil

}

func (r *GwRebuild) FileRreport() ([]byte, error) {

	b, err := r.retrieveGwFile(".xml")
	if err != nil {
		return nil, fmt.Errorf("report file not found")
	}
	return b, nil
}

func (r *GwRebuild) exe() error {

	app := os.Getenv("GWCLI")
	configini := os.Getenv("INICONFIG")
	xmlconfig := os.Getenv("XMLCONFIG")

	randConfigini := fmt.Sprintf("%s/%s/%s", INPUT, r.RandDir, CONFIGINI)
	randXmlconfig := fmt.Sprintf("%s/%s/%s", INPUT, r.RandDir, XMLCONFIG)

	cmd := exec.Command("cp", configini, randConfigini)
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("cp", xmlconfig, randXmlconfig)
	err = cmd.Run()
	if err != nil {
		return err
	}

	iniconf(randConfigini, r.RandDir)

	args := fmt.Sprintf("%s -config=%s -xmlconfig=%s", app, randConfigini, randXmlconfig)

	cmd = exec.Command("sh", "-c", args)
	var out bytes.Buffer
	cmd.Stdout = &out

	err = cmd.Run()
	log.Println(string(out.Bytes()))
	if err != nil {
		return err
	}

	return nil
}

func sdkVersion() string {
	app := os.Getenv("GWCLI")
	args := fmt.Sprintf("%s -v", app)
	cmd := exec.Command("sh", "-c", args)
	var out bytes.Buffer
	cmd.Stdout = &out

	cmd.Run()

	s := string(out.Bytes())

	log.Printf("\033[32m GW rebuild SDK version : %s\n", s)
	return s

}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

const (
	clean             = "the file is clean"
	cleaned           = " the file is clean by rebuild engine"
	unprocessableFile = "the file is can't be prcesssed by the rebuild engine"
	internalError     = "server internal error"
)

func (r *GwRebuild) RebuildStatus() error {

	//enum rebuild_request_body_return {REBUILD_UNPROCESSED=0, REBUILD_REBUILT=1, REBUILD_FAILED=2, REBUILD_ERROR=9};
	/* REBUILD_UNPROCESSED - to continue to unchanged content */
	/* REBUILD_REBUILT - to continue to rebuilt content */
	/* REBUILD_FAILED - to report error and use supplied error report */
	/* REBUILD_ERROR - to report  processing error */

	return r.GwparseLog()

}

type gwLogInfo struct {
	statusCode    string
	statusMessage string
}

func (r *GwRebuild) GwparseLog() error {
	b, err := r.FileLog()
	if err != nil {
		return err
	}
	if len(b) < 200 {
		r.statusMessage = parseStatus(b)
	} else {
		r.statusMessage = parseStatus(b[(len(b) - 200):])

	}
	return nil
}

func parseStatus(b []byte) string {
	sl := strings.Split(string(b), "\n")
	for _, s := range sl {
		statusdesc := parseCode(s)
		if statusdesc != "" {
			return statusdesc
		}
	}

	return "UNPROCESSABLE"

}

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

func checkfileExist() {
	if _, err := os.Stat("/path/to/whatever"); os.IsNotExist(err) {
		// path/to/whatever does not exist
	}
}

func (r GwRebuild) PrintStatus() string {

	s := r.statusMessage
	return s
}

func (r *GwRebuild) GwFileLog() ([]byte, error) {

	fileLog := fmt.Sprintf("%s/%s/%s", r.path, REBUILDOUTPUT, "glasswallCLIProcess.log")

	b, err := ioutil.ReadFile(fileLog)
	if err != nil {
		return nil, fmt.Errorf("glasswallCLIProcess.log fileLog file not found")
	}
	return b, nil

}

func (r *GwRebuild) FileLog() ([]byte, error) {

	b, err := r.retrieveGwFile(".log")
	if err != nil {
		return nil, fmt.Errorf("log file not found")
	}
	return b, nil

}

func (r *GwRebuild) retrieveGwFile(fileNameExt string) ([]byte, error) {

	pathManaged := fmt.Sprintf("%s/%s/%s/%s%s", r.path, REBUILDOUTPUT, MANAGED, r.FileName, fileNameExt)
	pathNonconforming := fmt.Sprintf("%s/%s/%s/%s%s", r.path, REBUILDOUTPUT, NONCONFORMING, r.FileName, fileNameExt)

	b, err := ioutil.ReadFile(pathManaged)
	if err != nil {
		b, err = ioutil.ReadFile(pathNonconforming)
		if err != nil {
			return nil, err
		}

	}
	return b, nil

}

func parseVersion(b []byte) string {
	sl := strings.Split(string(b), "\n")
	if len(sl) > 0 {
		return sl[0]
	}
	return ""
}
