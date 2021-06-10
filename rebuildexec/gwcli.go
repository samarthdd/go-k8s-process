package rebuildexec

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/k8-proxy/go-k8s-process/events"

	zlog "github.com/rs/zerolog/log"
)

const (
	APP           = "glasswallCLI"
	CONFIGINI     = "config.ini"
	XMLCONFIG     = "config.xml"
	MANAGED       = "Managed"
	NONCONFORMING = "NonConforming"
	INPUTKEY      = "inputLocation"
	OUTPUTKEY     = "outputLocation"
	SECTION       = "GWConfig"
	REBUILDINPUT  = "input"
	REBUILDOUTPUT = "output"
	FILETYPEKEY   = "fileType"
)

const (
	RebuildStatusInternalError = "INTERNAL ERROR"
	RebuildStatusClean         = "CLEAN"
	RebuildStatusCleaned       = "CLEANED"
	RebuildStatusUnprocessable = "UNPROCESSABLE"
	RebuildStatusExpired       = "SDK EXPIRED"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type GwRebuild struct {
	File     []byte
	FileId   string
	FileName string
	FileType string
	workDir  string

	cmp      policy
	cmpState bool

	statusMessage string
	args          string
	event         events.EventManager
	zipProc       zipProcess
	RebuiltFile   []byte
	ReportFile    []byte
	LogFile       []byte
	GwLogFile     []byte
	Metadata      []byte
}

func NewRebuild(file, cmp []byte, fileId, fileType, randDir, processDir string) GwRebuild {
	var err error
	fullpath := filepath.Join(processDir, randDir)
	randstr := RandStringRunes(16)

	ftype := GetContentType(file)

	if ftype == "zip" {
		fileType = "zip"
	}

	cmpState := false
	cmPolicy := policy{}
	if len(cmp) > 0 {
		cmpState = true
		cmPolicy, err = cmpJsonMarshal(cmp)
		zlog.Error().Err(err).Msg("error processing content management json file")

	}

	gwRebuild := GwRebuild{
		File:          file,
		FileId:        fileId,
		FileName:      randstr,
		cmp:           cmPolicy,
		cmpState:      cmpState,
		FileType:      fileType,
		workDir:       fullpath,
		statusMessage: RebuildStatusInternalError,
		event:         events.EventManager{FileId: fileId},
	}

	if gwRebuild.cmp.PolicyId == "" {
		gwRebuild.cmp.PolicyId = "00000000-0000-0000-0000-000000000000"
	}
	gwRebuild.event.NewDocument(gwRebuild.cmp.PolicyId)

	gwRebuild.event.FileTypeDetected(ftype)

	return gwRebuild
}

func (r *GwRebuild) RebuildSetup() error {
	var err error
	if r.FileType == "zip" {
		err = r.RebuildZipSetup()
	} else {
		err = r.RebuildFileSetup()
	}
	if err != nil {
		return err

	}

	return nil
}

func (r *GwRebuild) RebuildZipSetup() error {
	r.event.RebuildStarted()

	err := setupDirs(r.workDir)
	if err != nil {
		return err
	}

	err = r.copyTargetFile()
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s/%s", r.workDir, REBUILDINPUT, r.FileName)
	r.zipProc = zipProcess{
		workdir:   filepath.Dir(path),
		zipEntity: nil,
		ext:       "",
	}
	err = r.extractZip()
	if err != nil {
		return err
	}

	err = r.exe()
	if err != nil {
		return err
	}

	return nil

}

func (r *GwRebuild) RebuildFileSetup() error {
	r.event.RebuildStarted()

	err := setupDirs(r.workDir)
	if err != nil {
		return err
	}

	err = r.copyTargetFile()
	if err != nil {
		return err
	}
	err = r.exe()

	if err != nil {
		return err
	}

	return nil
}

func (r *GwRebuild) Yield() {

	if r.FileType == "zip" {
		r.zipAll("")
		r.zipAll(".xml")
		r.zipAll(".log")
	}
	errExp := r.CheckForExpire()
	if errExp != nil {
		r.FileType = "pdf"
		r.CheckIfExpired()

	}

	r.loadfFilesAfterProcess()

	r.rebuildStatus()
	r.event.RebuildCompleted(Gwoutcome(r.statusMessage))
	r.StopRecordEvent()
}

func (r *GwRebuild) StopRecordEventWithError() {
	r.event.RebuildCompleted(Gwoutcome(r.statusMessage))
	r.StopRecordEvent()
}

func (r *GwRebuild) copyTargetFile() error {
	path := fmt.Sprintf("%s/%s/%s", r.workDir, REBUILDINPUT, r.FileName)

	err := ioutil.WriteFile(path, r.File, 0666)
	return err
}

func (r *GwRebuild) CheckIfExpired() error {

	r.args = fmt.Sprintf("%s fileType=%s", r.args, "pdf")

	err := r.Execute()

	if err != nil {

		return err
	}

	return nil
}

func (r *GwRebuild) Clean() error {
	err := os.RemoveAll(r.workDir)
	return err
}

func (r *GwRebuild) loadfFilesAfterProcess() {
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

	r.GwLogFile, err = r.gwFileLog()
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
		return fmt.Errorf("fail to read ini file  %s", err)
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
		return fmt.Errorf("failed to save ini file : %s", err)

	}

	r.args = fmt.Sprintf("%s -config=%s -xmlconfig=%s", app, randConfigini, randXmlconfig)
	return nil

}

func (r *GwRebuild) Execute() error {
	_, err := gwCliExec(r.args)
	if err != nil {
		return err
	}

	return nil
}

func (r *GwRebuild) CheckForExpire() error {
	ext := ".log"
	if r.FileType == "zip" {
		ext = "log"
	}

	pathManaged := fmt.Sprintf("%s/%s/%s/%s%s", r.workDir, REBUILDOUTPUT, MANAGED, r.FileName, ext)
	pathNonconforming := fmt.Sprintf("%s/%s/%s/%s%s", r.workDir, REBUILDOUTPUT, NONCONFORMING, r.FileName, ext)

	if _, err := os.Stat(pathManaged); os.IsNotExist(err) {
		if _, err := os.Stat(pathNonconforming); os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (r *GwRebuild) rebuildStatus() {

	b := r.LogFile

	r.statusMessage = parseStatus(string(b))

	if r.FileType == "zip" {
		if r.statusMessage == RebuildStatusClean || r.statusMessage == RebuildStatusUnprocessable {
			r.statusMessage = RebuildStatusCleaned
		}
	}

}

func (r *GwRebuild) PrintStatus() string {

	s := r.statusMessage
	return s
}

func (r *GwRebuild) gwFileLog() ([]byte, error) {

	fileLog := fmt.Sprintf("%s/%s/%s", r.workDir, REBUILDOUTPUT, "glasswallCLIProcess.log")

	b, err := ioutil.ReadFile(fileLog)
	if err != nil {
		return nil, fmt.Errorf("glasswallCLIProcess.log  file not found")
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

func (r *GwRebuild) extractZip() error {
	err := r.zipProc.openZip(r.FileName)
	if err != nil {
		return err
	}

	os.Remove(filepath.Join(r.zipProc.workdir, r.FileName))

	return nil
}

func (r GwRebuild) zipAll(ext string) error {
	path := fmt.Sprintf("%s/%s", r.workDir, REBUILDOUTPUT)

	r.zipProc.ext = ext
	r.zipProc.workdir = path
	r.zipProc.readAllFilesExt(NONCONFORMING)
	r.zipProc.readAllFilesExt(MANAGED)

	if len(ext) > 1 {
		ext = ext[1:]
	}

	outName := fmt.Sprintf("%s%s", r.FileName, ext)
	r.zipProc.workdir = filepath.Join(path, MANAGED)
	err := r.zipProc.writeZip(outName)

	return err
}

func (r *GwRebuild) StopRecordEvent() error {
	b, err := r.event.MarshalJson()
	if err != nil {

		return err
	}
	r.Metadata = b
	return nil
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

func Gwoutcome(status string) string {
	switch status {

	case RebuildStatusCleaned, RebuildStatusClean:

		return "replace"
	case RebuildStatusUnprocessable:
		return "unmodified"
	case RebuildStatusExpired, RebuildStatusInternalError:
		return "failed"
	}
	return ""
}

func GetContentType(b []byte) string {
	if len(b) < 512 {
		return "UNKNOWN"
	}
	c := parseContnetType(http.DetectContentType(b[:511]))
	if c == "" {
		c = "UNKNOWN"
	}
	return c
}

func parseContnetType(s string) string {
	sl := strings.Split(s, "/")
	if len(sl) > 1 {
		return sl[1]
	}
	return s
}
