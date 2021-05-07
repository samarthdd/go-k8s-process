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
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
	rand.Seed(time.Now().UnixNano())
	os.MkdirAll(INPUT, 0777)
	printVersion()
}

type GwRebuild struct {
	File     []byte
	FileName string
	RandDir  string
	path     string
}

func New(file []byte, fileName, randDir string) GwRebuild {
	rebuilPath := filepath.Join(INPUT, randDir)
	os.MkdirAll(rebuilPath, 0777)

	return GwRebuild{file, fileName, randDir, rebuilPath}
}

func (r *GwRebuild) Rebuild() error {
	var err error

	path := fmt.Sprintf("%s/%s", r.path, r.FileName)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, r.File, 0666)
	if err != nil {
		return err
	}
	err = r.exe()
	if err != nil {
		return err
	}

	return nil
}

func (r *GwRebuild) Clean() error {
	err := os.RemoveAll(r.path)
	return err
}

func (r *GwRebuild) FileProcessed() ([]byte, error) {
	pathManaged := fmt.Sprintf("%s/%s/%s", r.path, MANAGED, r.FileName)
	pathNonconforming := fmt.Sprintf("%s/%s/%s", r.path, NONCONFORMING, r.FileName)

	b, err := ioutil.ReadFile(pathManaged)
	if err != nil {
		b, err = ioutil.ReadFile(pathNonconforming)
		if err != nil {
			return nil, err
		}

	}
	return b, nil

}

func (r *GwRebuild) FileRreport() ([]byte, error) {
	pathManaged := fmt.Sprintf("%s/%s/%s.xml", r.path, MANAGED, r.FileName)

	pathNonconforming := fmt.Sprintf("%s/%s/%s.xml", r.path, NONCONFORMING, r.FileName)

	b, err := ioutil.ReadFile(pathManaged)
	if err != nil {
		b, err = ioutil.ReadFile(pathNonconforming)
		if err != nil {
			return nil, fmt.Errorf("rebuild failed to process file")
		} else {
			return nil, fmt.Errorf("non comformed file ")

		}

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

func printVersion() {
	app := os.Getenv("GWCLI")
	args := fmt.Sprintf("%s -v", app)
	cmd := exec.Command("sh", "-c", args)
	var out bytes.Buffer
	cmd.Stdout = &out

	cmd.Run()

	log.Printf("\033[32m GW rebuild SDK version : %s\n", string(out.Bytes()))

}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
