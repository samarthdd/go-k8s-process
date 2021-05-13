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

	"github.com/go-ini/ini"

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

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

var rebuildSdkVersion string

func init() {
	rand.Seed(time.Now().UnixNano())
	os.MkdirAll(INPUT, 0777)
	rebuildSdkVersion = PrintVersion()
}

func GetSdkVersion() string {
	return rebuildSdkVersion
}

type GwRebuild struct {
	File     []byte
	FileName string
	FileType string
	workDir  string

	statusMessage string

	RebuiltFile []byte
	ReportFile  []byte
	LogFile     []byte
	GwLogFile   []byte
}

func New(file []byte, fileName, fileType, randDir string) GwRebuild {

	fullpath := filepath.Join(INPUT, randDir)

	gwRebuild := GwRebuild{
		File:     file,
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
		return err
	}

	path := fmt.Sprintf("%s/%s/%s", r.workDir, REBUILDINPUT, r.FileName)

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
	r.FileProcessed()

	if r.LogFile != nil {
		r.RebuildStatus()

	} else {
		r.FileType = "pdf"
		r.exe()

		if err != nil {
			r.statusMessage = "INTERNAL ERROR"

			return err
		}
		r.FileProcessed()

		r.RebuildStatus()

	}

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

	cmd = exec.Command("cp", xmlconfig, randXmlconfig)
	err = cmd.Run()
	if err != nil {
		return err
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

	err = inikey(sec, FILETYPEKEY, r.FileType)
	if err != nil {
		return err
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

func PrintVersion() string {

	app := os.Getenv("GWCLI")
	args := fmt.Sprintf("%s -v", app)

	b, err := gwCliExec(args)
	if err != nil {
		b = []byte(err.Error())
	}

	s := parseVersion(string(b))

	log.Printf("\033[32m GW rebuild SDK version : %s\n", s)
	return s
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

func (r *GwRebuild) RebuildStatus() {

	//enum rebuild_request_body_return {REBUILD_UNPROCESSED=0, REBUILD_REBUILT=1, REBUILD_FAILED=2, REBUILD_ERROR=9};
	/* REBUILD_UNPROCESSED - to continue to unchanged content */
	/* REBUILD_REBUILT - to continue to rebuilt content */
	/* REBUILD_FAILED - to report error and use supplied error report */
	/* REBUILD_ERROR - to report  processing error */
	b := r.LogFile

	r.statusMessage = parseStatus(string(b))

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
		return "EXPIRED"
	}
	return ""
}
