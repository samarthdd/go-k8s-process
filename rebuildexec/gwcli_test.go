package rebuildexec

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-ini/ini"
)

const (
	mainProjectDir          = "go-k8s-process"
	depDir                  = "sample"
	depDirTemp              = "/tmp/sample"
	CliTemp                 = "/tmp/sample/glasswallCLI"
	GwEnginePathUsrPathTemp = "/tmp/sample/libglasswall.classic.so"
	CliExecConfigIniTemp    = "/tmp/sample/testConfig.ini"
	CliExecXmlonfigTemp     = "/tmp/sample/testConfig.xml"

	sampleFilePath = "sample/sample.pdf"
	sampleFileName = "sample.pdf"
	randDir        = "ABCDIRGHRANDIBRA"
	RandDirLength  = 16

	GwCliPath    = "sdk-rebuild-eval/tools/command.line.tool/linux/glasswallCLI"
	GwEnginePath = "sdk-rebuild-eval/libs/rebuild/linux/libglasswall.classic.so"

	CliExecConfigIni  = "sample/testConfig.ini"
	CliExecXmlonfig   = "sample/testConfig.xml"
	rebuildInputTemp  = "/tmp/glrebuild/gwCliExecUnitTest/input"
	rebuildOutputTemp = "/tmp/glrebuild/gwCliExecUnitTest/output"

	SamplePdfPath = "sample/sample.pdf"
)

var (
	gwCliEnv        string
	xmlConfigEnv    string
	iniConfig       string
	mainProjectPath string
)

func init() {
	var exitt bool

	mainProjectPath, _ = os.Getwd()

	for i := 0; i < 20; i++ {
		if filepath.Base(mainProjectPath) == mainProjectDir {
			exitt = true
			break
		}
		mainProjectPath = filepath.Dir(mainProjectPath)
	}
	if !exitt {
		log.Fatalln("can't fin the main Project Dir")
	}

	projectDir := mainProjectPath

	os.MkdirAll(filepath.Join(mainProjectPath, rebuildInputTemp), 0777)
	os.MkdirAll(filepath.Join(mainProjectPath, rebuildOutputTemp), 0777)

	err := setupDep(projectDir)
	if err != nil {
		log.Println(err)
		log.Fatalln("can't setup depedencies : ", err)

	}

}

func setupDep(mainDir string) error {

	var out bytes.Buffer

	os.Setenv("LD_LIBRARY_PATH", filepath.Join(mainProjectPath, depDirTemp))
	log.Println(os.Getenv("LD_LIBRARY_PATH"))

	absouluteDepDir := filepath.Join(mainDir, depDir)

	cmd := exec.Command("cp", "-r", absouluteDepDir, filepath.Join(mainProjectPath, "tmp"))
	cmd.Stdout = &out
	err := cmd.Run()
	log.Println(string(out.Bytes()))
	if err != nil {
		log.Println(err)

		return err
	}
	os.Setenv("INICONFIG", filepath.Join(mainProjectPath, CliExecConfigIniTemp))
	os.Setenv("XMLCONFIG", filepath.Join(mainProjectPath, CliExecXmlonfigTemp))
	os.Setenv("GWCLI", filepath.Join(mainProjectPath, CliTemp))

	iniConfig = os.Getenv("INICONFIG")
	xmlConfigEnv = os.Getenv("XMLCONFIG")
	gwCliEnv = os.Getenv("GWCLI")

	absouluteCLi := filepath.Join(mainDir, GwCliPath)
	cmd = exec.Command("cp", absouluteCLi, filepath.Join(mainProjectPath, CliTemp))
	err = cmd.Run()
	if err != nil {
		return err
	}

	absouluteEngine := filepath.Join(mainDir, GwEnginePath)
	cmd = exec.Command("cp", absouluteEngine, filepath.Join(mainProjectPath, GwEnginePathUsrPathTemp))
	err = cmd.Run()
	if err != nil {
		return err
	}

	err = os.Chmod(filepath.Join(mainProjectPath, CliTemp), 0777)
	if err != nil {
		return err
	}
	return nil
}

func openSampleFile(filepath string) []byte {
	b, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		log.Fatal(err)
	}

	return b
}

func newRebuild(filepath string) GwRebuild {

	randPath := RandStringRunes(16)
	f := openSampleFile(filepath)

	rb := New(f, sampleFileName, "*", randPath)
	return rb
}

func moveFile(oldpath, newpath string) error {
	var out bytes.Buffer

	cmd := exec.Command("cp", oldpath, newpath)
	cmd.Stdout = &out
	err := cmd.Run()
	log.Println(string(out.Bytes()))
	if err != nil {
		log.Println(err)

		return err
	}
	return nil

}
func TestGwCliExec(t *testing.T) {

	pdfFilePath := filepath.Join(mainProjectPath, SamplePdfPath)
	inputPdffilePath := filepath.Join(mainProjectPath, rebuildInputTemp, sampleFileName)
	err := moveFile(pdfFilePath, inputPdffilePath)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bashErrorCode := 127
	rcUnknownFormatedDesc := fmt.Sprintf("%s : %v", unkownExitStatusCode, bashErrorCode)

	invalidGwCliEnv := "invalidGwCli"

	validArgs := fmt.Sprintf("%s -config=%s -xmlconfig=%s", gwCliEnv, iniConfig, xmlConfigEnv)
	InvalidArgs := fmt.Sprintf("%s -a -a", gwCliEnv)
	ConfigLoadFailureArgs := fmt.Sprintf("%s -config=nothing.ini -xmlconfig=nothing.xml", gwCliEnv)
	InvalidCliNameArgs := fmt.Sprintf("%s -config=%s -xmlconfig=%s", invalidGwCliEnv, iniConfig, xmlConfigEnv)

	// test case
	//rcINVALIDCOMMANDLINE
	//rcCONFIGLOADFAILURE
	//invalid CLi name
	execArgs := []struct {
		args   string
		result string
	}{
		{InvalidArgs, rcInvalidCommandLineDesc},
		{ConfigLoadFailureArgs, rcConfigLoadFailureDesc},
		{InvalidCliNameArgs, rcUnknownFormatedDesc},
		{validArgs, rcProcessingIssueDesc}, // test with wrong inputPath and outputPath in config.ini input
	}
	for _, v := range execArgs {
		_, err := gwCliExec(v.args)
		if err != nil {
			if err.Error() != v.result {
				t.Errorf("test fails eExpected %s got %s", v.result, err.Error())
			}
		} else {
			t.Errorf("test fails expected %s got %s", v.result, rcSucessDesc)
		}
	}

	changeIniconfigToValidPath(iniConfig)
	//test case with valid args
	_, err = gwCliExec(validArgs)
	if err != nil {
		t.Errorf("test fails expected %s got %s", rcSucessDesc, err.Error())
	}

	//test case rcDLLLOADFAILURE

	gwEnginepath := filepath.Join(mainProjectPath, GwEnginePathUsrPathTemp)

	os.Rename(gwEnginepath, gwEnginepath[:len(gwEnginepath)-1])
	_, err = gwCliExec(validArgs)
	if err != nil {
		if err.Error() != rcDllLoadFailureDesc {
			t.Errorf("test fails expected %s got %s", rcDllLoadFailureDesc, err.Error())
		}
	} else {
		t.Errorf("test fails expected %s got %s", rcDllLoadFailureDesc, rcSucessDesc)

	}
	os.Rename(gwEnginepath[:len(gwEnginepath)-1], gwEnginepath)

	//test case rcDLLLOADFAILURE

}

/*
func TestRebuild(t *testing.T) {

	//test with multiple file , conormed and managed
	var format = []string{
		".pdf",
		".pnv",
		".doc",
		".rar",
		"altred.pdf",
	}

	for _, v := range format {
		rb := newRebuild(v)
		err := rb.Rebuild()
		if err != nil {
			t.Errorf("")
		}

	}
}

*/
func TestPrintversion(t *testing.T) {
	//err := PrintVersion()
	//	if err != nil {
	//		t.Errorf("error printing version : %s", err)
	//	}

}

func TestCliExitStatus(t *testing.T) {
	rcUnknown := 5
	rcUnknownFormatedDesc := fmt.Sprintf("%s : %v", unkownExitStatusCode, rcUnknown)
	status := []struct {
		statusCode int
		statusDesc string
	}{
		{rcSucess, rcSucessDesc},
		{rcInvalidCommandLine, rcInvalidCommandLineDesc},
		{rcDllLoadFailure, rcDllLoadFailureDesc},
		{rcConfigLoadFailure, rcConfigLoadFailureDesc},
		{rcProcessingIssue, rcProcessingIssueDesc},
		{rcUnknown, rcUnknownFormatedDesc},
	}

	for _, v := range status {
		desc := CliExitStatus(v.statusCode)
		if desc != v.statusDesc {
			t.Errorf("errors expected %s got %s", v.statusDesc, desc)
		}

	}
}

func TestParseStatus(t *testing.T) {
	LogTest := []struct {
		log    string
		status string
	}{
		{LogFileClean, "CLEAN"},
		{LogFileCleaned, "CLEANED"},
		{LogFileExpir, "SDK EXPIRED"},
		{logFileUnprocessable, "UNPROCESSABLE"},
	}
	for _, v := range LogTest {
		res := parseStatus(v.log)
		if res != v.status {
			t.Errorf("fails expected %s got %s", v.status, res)
		}
	}
}

func TestParseLogExpir(t *testing.T) {
	LogTest := []struct {
		log    string
		status string
	}{
		{LogFileExpir, "SDK EXPIRED"},
		{LogFileClean, ""},
	}
	for _, v := range LogTest {
		res := parseLogExpir(v.log)
		if res != v.status {
			t.Errorf("fails expected %s got %s", v.status, res)
		}
	}
}

func TestParseVersion(t *testing.T) {
	validVersionOutput := `1.221
SUCCESS
`
	nonValidVersionOutput := "error no such command"
	emptyOutput := ""

	versionTest := []struct {
		text    string
		version string
	}{
		{validVersionOutput, "1.221"},
		{nonValidVersionOutput, "error no such command"},
		{emptyOutput, ""},
	}
	for _, v := range versionTest {
		res := parseVersion(v.text)
		if res != v.version {
			if v.version == "" {

				t.Errorf("fails expected empty string got %s", res)
			} else {
				t.Errorf("fails expected %s got %s", v.version, res)

			}

		}
	}
}

func TestRebuildFile(t *testing.T) {

	f, _ := ioutil.ReadFile(filepath.Join(mainProjectPath, depDirTemp, "sample.pdf"))
	randPath := RandStringRunes(16)
	fd := New(f, "samplee.pdf", "*", randPath)
	err := fd.Rebuild()
	log.Printf("\033[34m rebuild status is  : %s\n", fd.PrintStatus())

	if err != nil {
		t.Error(err)

	}

}

func changeIniconfigToValidPath(path string) {
	cfg, err := ini.Load(path)
	if err != nil {
		log.Printf("Fail to read ini file  %s", err)
	}

	sec := cfg.Section(SECTION)

	inputValue := filepath.Join(mainProjectPath, rebuildInputTemp)
	err = inikey(sec, INPUTKEY, inputValue)
	if err != nil {
		log.Printf("Fail to read ini key  %s", err)
	}

	outputValue := filepath.Join(mainProjectPath, rebuildOutputTemp)
	err = inikey(sec, OUTPUTKEY, outputValue)
	if err != nil {
		log.Printf("Fail to read ini key  %s", err)
	}

	err = cfg.SaveTo(path)
	if err != nil {
		log.Printf("Fail to save ini file : %s", err)

	}
}

func TestRebuildZip(t *testing.T) {

	zipPath := filepath.Join(mainProjectPath, depDirTemp, "nested.zip")

	f, _ := ioutil.ReadFile(zipPath)
	randPath := RandStringRunes(16)
	fd := New(f, "nested.zip", "zip", randPath)
	err := fd.Rebuild()
	log.Printf("\033[34m rebuild status a is  : %s\n", fd.PrintStatus())

	if err != nil {
		t.Error(err)

	}

}

// test GwCli failure case
// test GwCli with multiple file type
// add nonconfomed states
// add benchmark and processing info
