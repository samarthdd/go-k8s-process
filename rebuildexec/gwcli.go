package rebuildexec

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func init() {
	rand.Seed(time.Now().UnixNano())
	os.MkdirAll(INPUT, 0777)
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
	Metadata    []byte
}

func New(file []byte, fileName, fileType, randDir string) GwRebuild {

	fullpath := filepath.Join(INPUT, randDir)

	if len(file) > 512 {
		c := http.DetectContentType(file[:511])
		if c == "application/zip" {
			fileType = "zip"
		}

	}

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
	ev.NewDocument("00000000-0000-0000-0000-000000000000")

	if r.statusMessage != "INTERNAL ERROR" && r.statusMessage != "SDK EXPIRED" {

		fileType := parseContnetType(http.DetectContentType(r.File[:511]))

		ev.FileTypeDetected(fileType)
		gwoutcome := gwoutcome(r.statusMessage)

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

func gwoutcome(status string) string {
	switch status {
	case "CLEAN":
		return "unmodified"
	case "CLEANED":
		return "replace"
	case "UNPROCESSABLE":
		return "unmodified"
	case "SDK EXPIRED", "INTERNAL ERROR":
		return "failed"
	}
	return ""
}
