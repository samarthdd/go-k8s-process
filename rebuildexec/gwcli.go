package rebuildexec

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-ini/ini"
	"github.com/k8-proxy/go-k8s-process/events"

	zlog "github.com/rs/zerolog/log"
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
	FILETYPEKEY   = "fileType"
)

var once sync.Once
var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

var rebuildSdkVersion string

func init() {
	rand.Seed(time.Now().UnixNano())
	os.MkdirAll(INPUT, 0777)
}

func GetSdkVersion() string {
	once.Do(func() {
		rebuildSdkVersion = GetVersion()
	})

	return rebuildSdkVersion
}

type GwRebuild struct {
	File     []byte
	FileName string
	FileType string
	workDir  string

	cmp           policy
	cmpState      bool
	statusMessage string

	RebuiltFile []byte
	ReportFile  []byte
	LogFile     []byte
	GwLogFile   []byte
	Metadata    []byte
}

func New(file, cmp []byte, fileName, fileType, randDir string) GwRebuild {

	fullpath := filepath.Join(INPUT, randDir)

	if len(file) > 512 {
		c := http.DetectContentType(file[:511])
		if c == "application/zip" {
			fileType = "zip"
		}

	}

	cmpState := false
	cmPolicy := policy{}
	if len(cmp) > 0 {
		cmpState = true
		cmPolicy, _ = cmpJsonMarshal(cmp)

	}

	gwRebuild := GwRebuild{
		File:     file,
		cmp:      cmPolicy,
		cmpState: cmpState,
		FileName: fileName,
		FileType: fileType,
		workDir:  fullpath,
	}

	return gwRebuild
}

func setupDirs(workDir string) error {

	err := os.MkdirAll(workDir, 0777)
	if err != nil {
		return err
	}

	inputRebuildpath := filepath.Join(workDir, REBUILDINPUT)
	err = os.MkdirAll(inputRebuildpath, 0777)
	if err != nil {
		return err
	}
	outputRebuildpath := filepath.Join(workDir, REBUILDOUTPUT)

	err = os.MkdirAll(outputRebuildpath, 0777)
	if err != nil {
		return err
	}
	return nil
}

func (r *GwRebuild) Rebuild() error {

	err := setupDirs(r.workDir)
	if err != nil {
		r.statusMessage = "INTERNAL ERROR"
		r.event()

		return err
	}

	path := fmt.Sprintf("%s/%s/%s", r.workDir, REBUILDINPUT, r.FileName)

	err = ioutil.WriteFile(path, r.File, 0666)
	if err != nil {
		r.statusMessage = "INTERNAL ERROR"
		r.event()

		return err
	}

	if r.FileType == "zip" {
		zipProc := zipProcess{
			workdir:   filepath.Dir(path),
			zipEntity: nil,
			ext:       "",
		}
		err = r.extractZip(&zipProc)
		if err != nil {
			r.statusMessage = "INTERNAL ERROR"
			r.event()

			return err
		}

		err = r.exe()
		if err != nil {
			r.statusMessage = "INTERNAL ERROR"
			r.event()

			return err
		}

		r.zipAll(zipProc, "")
		r.zipAll(zipProc, ".xml")
		r.zipAll(zipProc, ".log")

	} else {

		err = r.exe()

		if err != nil {
			r.statusMessage = "INTERNAL ERROR"
			r.event()

			return err
		}
	}

	r.FileProcessed()

	if r.LogFile != nil {
		r.RebuildStatus()

	} else {
		r.FileType = "pdf"
		r.exe()

		if err != nil {
			r.statusMessage = "INTERNAL ERROR"
			r.event()

			return err
		}
		r.FileProcessed()

		r.RebuildStatus()

	}
	r.event()

	return nil
}

func (r *GwRebuild) Clean() error {
	err := os.RemoveAll(r.workDir)
	return err
}

func (r *GwRebuild) FileProcessed() {
	var err error
	r.RebuiltFile, err = r.retrieveGwFile("")
	if err != nil {
		zlog.Error().Err(err).Msg("processed file not found")
	}

	r.ReportFile, err = r.retrieveGwFile(".xml")
	if err != nil {
		zlog.Error().Err(err).Msg("report file not found")
	}

	r.LogFile, err = r.retrieveGwFile(".log")
	if err != nil {
		zlog.Error().Err(err).Msg("log file not found")
	}

	r.GwLogFile, err = r.GwFileLog()
	if err != nil {
		zlog.Error().Err(err).Msg("gw log file not found")
	}

}

func (r *GwRebuild) exe() error {

	app := os.Getenv("GWCLI")
	configini := os.Getenv("INICONFIG")
	xmlconfig := os.Getenv("XMLCONFIG")

	randConfigini := fmt.Sprintf("%s/%s", r.workDir, CONFIGINI)
	randXmlconfig := fmt.Sprintf("%s/%s", r.workDir, XMLCONFIG)

	cmd := exec.Command("cp", configini, randConfigini)
	err := cmd.Run()
	if err != nil {
		return err
	}

	if r.cmpState {
		xmlCmp, _ := r.cmp.cmpXmlconv()

		errWrite := ioutil.WriteFile(randXmlconfig, xmlCmp, 0777)
		if errWrite != nil {
			return errWrite
		}
	} else {

		cmd = exec.Command("cp", xmlconfig, randXmlconfig)
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	cfg, err := ini.Load(randConfigini)
	if err != nil {
		return fmt.Errorf("Fail to read ini file  %s", err)
	}

	sec := cfg.Section(SECTION)

	inputValue := filepath.Join(r.workDir, REBUILDINPUT)
	err = inikey(sec, INPUTKEY, inputValue)
	if err != nil {
		return err
	}

	outputValue := filepath.Join(r.workDir, REBUILDOUTPUT)
	err = inikey(sec, OUTPUTKEY, outputValue)
	if err != nil {
		return err
	}

	if r.FileType != "zip" {
		err = inikey(sec, FILETYPEKEY, r.FileType)
		if err != nil {
			return err
		}
	}

	err = cfg.SaveTo(randConfigini)
	if err != nil {
		return fmt.Errorf("Fail to save ini file : %s", err)

	}

	args := fmt.Sprintf("%s -config=%s -xmlconfig=%s", app, randConfigini, randXmlconfig)

	b, err := gwCliExec(args)
	if err != nil {
		return err
	}

	log.Printf("\033[32m %s", string(b))

	return nil
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

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func (r *GwRebuild) RebuildStatus() {

	//enum rebuild_request_body_return {REBUILD_UNPROCESSED=0, REBUILD_REBUILT=1, REBUILD_FAILED=2, REBUILD_ERROR=9};
	/* REBUILD_UNPROCESSED - to continue to unchanged content */
	/* REBUILD_REBUILT - to continue to rebuilt content */
	/* REBUILD_FAILED - to report error and use supplied error report */
	/* REBUILD_ERROR - to report  processing error */
	b := r.LogFile

	r.statusMessage = parseStatus(string(b))

	if r.FileType == "zip" {
		if r.statusMessage == "CLEAN" || r.statusMessage == "UNPROCESSABLE" {
			r.statusMessage = "CLEANED"
		}
	}

}

func (r *GwRebuild) GwparseLog(b []byte) {

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

func (r GwRebuild) PrintStatus() string {

	s := r.statusMessage
	return s
}

func (r *GwRebuild) GwFileLog() ([]byte, error) {

	fileLog := fmt.Sprintf("%s/%s/%s", r.workDir, REBUILDOUTPUT, "glasswallCLIProcess.log")

	b, err := ioutil.ReadFile(fileLog)
	if err != nil {
		return nil, fmt.Errorf("glasswallCLIProcess.log fileLog file not found")
	}
	return b, nil

}

func (r *GwRebuild) retrieveGwFile(fileNameExt string) ([]byte, error) {
	if r.FileType == "zip" {
		if len(fileNameExt) > 1 {
			fileNameExt = fileNameExt[1:]
		}
	}
	pathManaged := fmt.Sprintf("%s/%s/%s/%s%s", r.workDir, REBUILDOUTPUT, MANAGED, r.FileName, fileNameExt)
	pathNonconforming := fmt.Sprintf("%s/%s/%s/%s%s", r.workDir, REBUILDOUTPUT, NONCONFORMING, r.FileName, fileNameExt)

	b, err := ioutil.ReadFile(pathManaged)
	if err != nil {
		b, err = ioutil.ReadFile(pathNonconforming)
		if err != nil {
			return nil, err
		}

	}
	return b, nil

}

func parseVersion(b string) string {
	sl := strings.Split(string(b), "\n")

	if len(sl) > 0 {
		return sl[0]
	}
	return ""
}

func gwCliExec(args string) ([]byte, error) {
	cmd := exec.Command("sh", "-c", args)
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitStarusDesc := CliExitStatus(exitError.ExitCode())
			return nil, fmt.Errorf(exitStarusDesc)
		}
		return nil, err
	}

	b := out.Bytes()
	return b, nil
}

const (
	rcSucess = iota
	rcInvalidCommandLine
	rcDllLoadFailure
	rcConfigLoadFailure
	rcProcessingIssue
)

const (
	rcSucessDesc             = "rcSucessDesc : Test completed successfully"
	rcInvalidCommandLineDesc = "rcInvalidCommandLineDesc : Command line argument is invalid"
	rcDllLoadFailureDesc     = "rcDllLoadFailureDesc :Problem loading the DLL/Shared library"
	rcConfigLoadFailureDesc  = "rcConfigLoadFailureDesc : Problem loading the specified configuration file"
	rcProcessingIssueDesc    = "rcProcessingIssueDesc : Problem processing the specified files"
	unkownExitStatusCode     = "unknown exit status code"
)

func CliExitStatus(errCode int) string {
	switch errCode {
	case rcSucess:
		return rcSucessDesc
	case rcInvalidCommandLine:
		return rcInvalidCommandLineDesc
	case rcDllLoadFailure:
		return rcDllLoadFailureDesc
	case rcConfigLoadFailure:
		return rcConfigLoadFailureDesc
	case rcProcessingIssue:
		return rcProcessingIssueDesc
	default:
		return fmt.Sprintf("%s : %v", unkownExitStatusCode, errCode)

	}

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

func (r *GwRebuild) extractZip(z *zipProcess) error {
	err := z.openZip(r.FileName)
	if err != nil {
		return err
	}

	err = os.Remove(filepath.Join(z.workdir, r.FileName))

	return nil
}

func (r *GwRebuild) zipAll(z zipProcess, ext string) error {
	path := fmt.Sprintf("%s/%s", r.workDir, REBUILDOUTPUT)

	z.ext = ext
	z.workdir = path
	z.readAllFilesExt(NONCONFORMING)
	z.readAllFilesExt(MANAGED)

	if len(ext) > 1 {
		ext = ext[1:]
	}

	outName := fmt.Sprintf("%s%s", r.FileName, ext)
	z.workdir = filepath.Join(path, MANAGED)
	err := z.writeZip(outName)

	return err
}
func (r *GwRebuild) event() error {
	var ev events.EventManager
	ev = events.EventManager{FileId: r.FileName}

	policyId := "00000000-0000-0000-0000-000000000000"
	if r.cmp.PolicyId != "" {
		policyId = r.cmp.PolicyId
	}
	ev.NewDocument(policyId)

	if r.statusMessage != "INTERNAL ERROR" && r.statusMessage != "SDK EXPIRED" {

		fileType := parseContnetType(http.DetectContentType(r.File[:511]))

		ev.FileTypeDetected(fileType)
		gwoutcome := Gwoutcome(r.statusMessage)

		ev.RebuildStarted()
		ev.RebuildCompleted(gwoutcome)
	}

	b, err := ev.MarshalJson()
	if err != nil {

		return err
	}
	r.Metadata = b
	return nil
}

func Gwoutcome(status string) string {
	switch status {
	case "CLEAN", "CLEANED":
		return "replace"
	case "UNPROCESSABLE":
		return "unmodified"
	case "SDK EXPIRED", "INTERNAL ERROR":
		return "failed"
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
