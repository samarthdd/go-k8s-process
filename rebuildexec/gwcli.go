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
	PATH          = "./dep"
	INPUT         = "/tmp/glrebuild"
	MANAGED       = "Managed"
	NONCONFORMING = "NonConforming"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	os.Mkdir(INPUT, 0777)
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

type GwRebuild struct {
	File     []byte
	FileName string
	path     string
}

func New(f []byte, n string) GwRebuild {
	return GwRebuild{f, n, ""}
}

func (r *GwRebuild) Rebuild() error {
	var err error
	//r.path, err = os.MkdirTemp(INPUT, "gl")
	r.path = INPUT

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

func (r *GwRebuild) clean() {
	logm := fmt.Sprintf("%s/%s/%s.log", r.path, MANAGED, r.FileName)

	logn := fmt.Sprintf("%s/%s/%s.log", r.path, NONCONFORMING, r.FileName)

	fpath := fmt.Sprintf("%s/%s", r.path, r.FileName)

	os.Remove(logm)

	os.Remove(logn)

	os.Remove(fpath)

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
		os.Remove(pathNonconforming)

	} else {
		os.Remove(pathManaged)
	}
	r.clean()
	return b, nil

}

func (r *GwRebuild) FileRreport() ([]byte, error) {
	pathManaged := fmt.Sprintf("%s/%s/%s.xml", r.path, MANAGED, r.FileName)

	pathNonconforming := fmt.Sprintf("%s/%s/%s.xml", r.path, NONCONFORMING, r.FileName)

	b, err := ioutil.ReadFile(pathManaged)
	if err != nil {
		b, err = ioutil.ReadFile(pathNonconforming)
		if err != nil {
			return nil, err
		}
		os.Remove(pathNonconforming)

	} else {
		os.Remove(pathManaged)

	}
	r.clean()
	return b, nil

}

func (r *GwRebuild) exe() error {
	envr := os.Getenv("IN_CONTAINER")
	log.Println("in container", envr)

	path, err := filepath.Abs(PATH)

	if err != nil {
		return err
	}

	if envr == "true" {
		path = PATH[1:]
	}
	log.Println("path", path)

	app := fmt.Sprintf("%s/%s", path, APP)
	configini := fmt.Sprintf("%s/%s", path, CONFIGINI)
	xmlconfig := fmt.Sprintf("%s/%s", path, XMLCONFIG)

	log.Println("path", app)
	log.Println("path", configini)
	log.Println("path", xmlconfig)

	args := fmt.Sprintf("%s -config=%s -xmlconfig=%s", app, configini, xmlconfig)

	cmd := exec.Command("sh", "-c", args)
	var out bytes.Buffer
	cmd.Stdout = &out

	log.Println(path)
	//cmd.Dir = path
	err = cmd.Run()
	log.Println(string(out.Bytes()))
	if err != nil {
		return err
	}

	return nil
}
