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
)

const (
	mainProjectDir       = "go-k8s-process"
	depDir               = "sample"
	TempdepDir           = "/tmp/sample"
	CliTemp              = "/tmp/sample/glasswallCLI"
	GwEnginePathUsrPath  = "/tmp/sample/libglasswall.classic.so"
	CliExecConfigIniTemp = "/tmp/sample/testConfig.ini"
	CliExecXmlonfigTemp  = "/tmp/sample/testConfig.xml"

	sampleFilePath = "sample/sample.pdf"
	sampleFileName = "sample.pdf"
	randDir        = "ABCDIRGHRANDIBRA"
	RandDirLength  = 16

	GwCliPath    = "sdk-rebuild-eval/tools/command.line.tool/linux/glasswallCLI"
	GwEnginePath = "sdk-rebuild-eval/libs/rebuild/linux/libglasswall.classic.so"

	CliExecConfigIni = "sample/testConfig.ini"
	CliExecXmlonfig  = "sample/testConfig.xml"
	rbuildInput      = "/tmp/glrebuild/gwCliExecUnitTest/input"
	rbuildOutput     = "/tmp/glrebuild/gwCliExecUnitTest/output"

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

	os.MkdirAll("/tmp/glrebuild/gwCliExecUnitTest/input", 0777)
	os.MkdirAll("/tmp/glrebuild/gwCliExecUnitTest/output", 0777)

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

	err := setupDep(projectDir)
	if err != nil {
		log.Println(err)
		log.Fatalln("can't setup depedencies : ", err)

	}

}

func mainPorojectDir() string {
	return "void"
}

func setupDep(mainDir string) error {

	var out bytes.Buffer

	os.Setenv("LD_LIBRARY_PATH", TempdepDir)
	log.Println(os.Getenv("LD_LIBRARY_PATH"))

	absouluteDepDir := filepath.Join(mainDir, depDir)

	cmd := exec.Command("cp", "-r", absouluteDepDir, TempdepDir)
	cmd.Stdout = &out
	err := cmd.Run()
	log.Println(string(out.Bytes()))
	if err != nil {
		log.Println(err)

		return err
	}
	os.Setenv("INICONFIG", CliExecConfigIniTemp)
	os.Setenv("XMLCONFIG", CliExecXmlonfigTemp)
	os.Setenv("GWCLI", CliTemp)

	iniConfig = os.Getenv("INICONFIG")
	xmlConfigEnv = os.Getenv("XMLCONFIG")
	gwCliEnv = os.Getenv("GWCLI")

	absouluteCLi := filepath.Join(mainDir, GwCliPath)
	cmd = exec.Command("cp", absouluteCLi, CliTemp)
	err = cmd.Run()
	if err != nil {
		return err
	}

	absouluteEngine := filepath.Join(mainDir, GwEnginePath)
	cmd = exec.Command("cp", absouluteEngine, GwEnginePathUsrPath)
	err = cmd.Run()
	if err != nil {
		return err
	}

	err = os.Chmod(CliTemp, 0777)
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
	inputPdffilePath := filepath.Join(rbuildInput, sampleFileName)
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
	}
	for _, v := range execArgs {
		_, err := gwCliExec(v.args)
		if err != nil {
			if err.Error() != v.result {
				t.Errorf("test fails expected %s got %s", v.result, err.Error())
			}
		} else {
			t.Errorf("test fails expected %s got %s", v.result, rcSucessDesc)
		}
	}

	//test case with valid args
	_, err = gwCliExec(validArgs)
	if err != nil {
		t.Errorf("test fails expected %s got %s", rcSucessDesc, err.Error())
	}

	//test case rcDLLLOADFAILURE

	os.Rename(GwEnginePathUsrPath, GwEnginePathUsrPath[:len(GwEnginePathUsrPath)-1])
	_, err = gwCliExec(validArgs)
	if err != nil {
		if err.Error() != rcDllLoadFailureDesc {
			t.Errorf("test fails expected %s got %s", rcDllLoadFailureDesc, err.Error())
		}
	} else {
		t.Errorf("test fails expected %s got %s", rcDllLoadFailureDesc, rcSucessDesc)

	}
	os.Rename(GwEnginePathUsrPath[:len(GwEnginePathUsrPath)-1], GwEnginePathUsrPath)

	//test case rcDLLLOADFAILURE

}
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

func TestFileProcessed(t *testing.T) {
	//rb := newRebuild()

}

func TestFileReport(t *testing.T) {
	if false {
		t.Errorf("er")
	}
}

func TestClean(t *testing.T) {

}

func TestExe(t *testing.T) {

}

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
		{logFileUnprocessable, "UNPROCESSABLE"},
	}
	for _, v := range LogTest {
		res := parseStatus(v.log)
		if res != v.status {
			t.Errorf("fails expected %s got %s", v.status, res)
		}
	}
}

/*
func TestParseLogExpir(t *testing.T) {
	LogTest := []struct {
		log    string
		status bool
	}{
		{LogFileExpir, "EXPIRED REBUILD SDK"},
		{LogFileClean, ""},
	}
	for _, v := range LogTest {
		res := parseLogExpir(v.log)
		if res != v.status {
			t.Errorf("fails expected %s got %s", v.status, res)
		}
	}
}
*/
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

	f, _ := ioutil.ReadFile("/home/ibrahim/Desktop/demopdf/sample.pdf")
	randPath := RandStringRunes(16)
	fd := New(f, "samplee.pdf", "*", randPath)
	err := fd.Rebuild()
	log.Printf("\033[34m rebuild status is  : %s\n", fd.PrintStatus())

	if err != nil {
		t.Error(err)

	}

}

// test GwCli failure case
// test GwCli with multiple file type
// add nonconfomed states
// add benchmark and processing info
