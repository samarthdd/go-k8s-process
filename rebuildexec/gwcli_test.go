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
	log.Println(out.String())
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

func moveFile(oldpath, newpath string) error {
	var out bytes.Buffer

	cmd := exec.Command("cp", oldpath, newpath)
	cmd.Stdout = &out
	err := cmd.Run()
	log.Println(out.String())
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

func TestRebuild(t *testing.T) {

	processDir := filepath.Join(mainProjectPath, "/tmp/glrebuild")

	files := []struct {
		Name   string
		Status string
	}{
		{"sample.pdf", RebuildStatusCleaned},
		{"sample.jpg", RebuildStatusCleaned},
		{"sample.doc", RebuildStatusClean},
		{"unprocessable.jpg", RebuildStatusUnprocessable},
		{"nested.zip", RebuildStatusCleaned},
	}

	path := filepath.Join(mainProjectPath, depDirTemp)
	for i := range files {
		f, err := ioutil.ReadFile(filepath.Join(path, files[i].Name))
		if err != nil {
			t.Error(err)
		}

		randPath := RandStringRunes(16)
		fd := NewRebuild(f, nil, files[i].Name, "*", randPath, processDir)
		err = fd.RebuildSetup()
		if err != nil {
			t.Error(err)
		}
		err = fd.Execute()
		if err != nil {
			t.Error(err)
		}
		fd.Yield()

		if fd.PrintStatus() != files[i].Status {
			t.Errorf("errors %s expected %s got %s", files[i].Name, files[i].Status, fd.PrintStatus())

		}
		if fd.RebuiltFile == nil && files[i].Status != RebuildStatusUnprocessable {
			t.Error("rebuilt file not found")

		}
		if fd.ReportFile == nil {
			t.Error("report file not found")
		}
		if fd.LogFile == nil {
			t.Error("Log  file not found")
		}
		if fd.GwLogFile == nil {
			t.Error("gw Log file not found")

		}
		if fd.Metadata == nil {
			t.Error("metadata  file not found")

		}
		//	fd.Clean()

	}

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
	processDir := filepath.Join(mainProjectPath, "/tmp/glrebuild")

	zipPath := filepath.Join(mainProjectPath, depDirTemp, "nested.zip")

	f, _ := ioutil.ReadFile(zipPath)
	randPath := RandStringRunes(16)

	fd := NewRebuild(f, nil, "nested.zip", "zip", randPath, processDir)
	err := fd.RebuildSetup()

	if err != nil {
		t.Error(err)

	}

	err = fd.Execute()
	if err != nil {
		t.Error(err)
	}
	fd.Yield()

	if fd.PrintStatus() != "CLEANED" {
		t.Errorf("errors %s expected %s got %s", "RebuildZip", "CLEANED", fd.PrintStatus())

	}
	fd.Clean()
}

func TestContentManagmentPolicy(t *testing.T) {
	processDir := filepath.Join(mainProjectPath, "/tmp/glrebuild")

	Path := filepath.Join(mainProjectPath, depDirTemp, "sample.pdf")

	f, _ := ioutil.ReadFile(Path)

	cmpPath := filepath.Join(mainProjectPath, depDirTemp, "cmp.json")
	cmp, _ := ioutil.ReadFile(cmpPath)
	cmp = bytes.TrimPrefix(cmp, []byte("\xef\xbb\xbf"))

	randPath := RandStringRunes(16)

	fd := NewRebuild(f, cmp, "sample.pdf", "*", randPath, processDir)
	err := fd.RebuildSetup()

	if err != nil {
		t.Error(err)

	}

	err = fd.Execute()
	if err != nil {
		t.Error(err)
	}
	fd.Yield()

	if fd.PrintStatus() != "CLEAN" {
		t.Errorf("errors %s expected %s got %s", "TestContentManagmentPolicy", "CLEANED", fd.PrintStatus())

	}
	if fd.cmp.PolicyId != "1c47fecd-be3c-48c8-9e8d-ee4abc7eafef" {
		t.Errorf("errors %s expected %s got %s", "TestContentManagmentPolicy", "1c47fecd-be3c-48c8-9e8d-ee4abc7eafef", fd.cmp.PolicyId)

	}
	fd.Clean()
}

func TestRebuildFileInternalError(t *testing.T) {
	os.Setenv("GWCLI", "NONVALIDPATH")
	processDir := filepath.Join(mainProjectPath, "/tmp/glrebuild")

	f, _ := ioutil.ReadFile(filepath.Join(mainProjectPath, depDirTemp, "sample.pdf"))
	randPath := RandStringRunes(16)
	fd := NewRebuild(f, nil, "samplee.pdf", "*", randPath, processDir)
	err := fd.RebuildSetup()

	if err != nil {
		t.Error(err)

	}
	err = fd.Execute()
	if err == nil {
		t.Errorf("expected CLI error with exit code 1 ")
	}
	if fd.PrintStatus() != RebuildStatusInternalError {
		t.Errorf("error expected %s got %s ", RebuildStatusInternalError, fd.PrintStatus())

	}
	t.Cleanup(func() {
		os.Setenv("GWCLI", filepath.Join(mainProjectPath, CliTemp))

	})
}
func TestExpired(t *testing.T) {
	currentTag, err := getCurrentGitTag()
	if err != nil {
		t.Error(err)

	}
	if len(currentTag) > 1 {
		currentTag = currentTag[:len(currentTag)-1]
	}
	err = checkoutTag("1.191")
	if err != nil {
		t.Error(err)

	}
	errSetup := setupExpireLibrary()
	if errSetup != nil {
		t.Error(errSetup)

	}
	processDir := filepath.Join(mainProjectPath, "/tmp/glrebuild")

	Path := filepath.Join(mainProjectPath, depDirTemp, "sample.pdf")

	f, _ := ioutil.ReadFile(Path)
	randPath := RandStringRunes(16)

	fd := NewRebuild(f, nil, "sample.pdf", "*", randPath, processDir)
	err = fd.RebuildSetup()

	if err != nil {
		t.Error(err)

	}

	err = fd.Execute()
	if err != nil {
		t.Error(err)
	}
	fd.Yield()

	if fd.PrintStatus() != RebuildStatusExpired {
		t.Errorf("errors %s expected %s got %s", "TestExpired", RebuildStatusExpired, fd.PrintStatus())

	}
	fd.Clean()
	t.Cleanup(func() {
		checkoutTag(currentTag)
		os.Setenv("LD_LIBRARY_PATH", filepath.Join(mainProjectPath, depDirTemp))
		os.Setenv("GWCLI", filepath.Join(mainProjectPath, "tmp", "expire-cli-path", "glasswallCLI"))
	})

}
func setupExpireLibrary() error {

	os.MkdirAll(filepath.Join(mainProjectPath, "tmp", "expire-cli-path"), 0777)

	expireCliPath := filepath.Join(mainProjectPath, "tmp", "expire-cli-path", "glasswallCLI")
	expireSoPath := filepath.Join(mainProjectPath, "tmp", "expire-cli-path", "libglasswall.classic.so")
	GwCliPath1191 := filepath.Join(mainProjectPath, GwCliPath)
	GwEnginePath1991 := filepath.Join(mainProjectPath, GwEnginePath)
	cmd := exec.Command("cp", GwCliPath1191, expireCliPath)
	err := cmd.Run()
	if err != nil {
		return err
	}
	cmd = exec.Command("cp", GwEnginePath1991, expireSoPath)
	err = cmd.Run()
	if err != nil {
		return err
	}
	err = os.Chmod(expireCliPath, 0777)
	if err != nil {
		return err
	}
	os.Setenv("LD_LIBRARY_PATH", filepath.Join(mainProjectPath, "tmp", "expire-cli-path"))
	os.Setenv("GWCLI", expireCliPath)

	return nil
}

func checkoutTag(tag string) error {

	absouluteEngine := filepath.Join(mainProjectPath, "sdk-rebuild-eval")

	cmd := exec.Command("git", "checkout", tag)
	cmd.Dir = absouluteEngine
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func getCurrentGitTag() (string, error) {

	absouluteEngine := filepath.Join(mainProjectPath, "sdk-rebuild-eval")

	var buf bytes.Buffer

	cmd := exec.Command("git", "describe", "--tags")
	cmd.Stdout = &buf
	cmd.Dir = absouluteEngine
	err := cmd.Run()
	if err != nil {
		log.Println(buf.String())
		return "", err
	}
	return buf.String(), nil
}

func TestClean(t *testing.T) {
	t.Cleanup(func() {
		path := filepath.Join(mainProjectPath, "tmp")
		os.RemoveAll(path)
	})
}
