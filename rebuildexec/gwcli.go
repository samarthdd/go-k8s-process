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

	"github.com/go-ini/ini"
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

type GwRebuild struct {
	File     []byte
	FileName string
	Lastpath string
	path     string
}

func init() {
	rand.Seed(time.Now().UnixNano())
	os.MkdirAll(INPUT, 0777)
	printVersion()
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

func New(f []byte, n, l string) GwRebuild {
	p := filepath.Join(INPUT, l)
	os.MkdirAll(p, 0777)

	return GwRebuild{f, n, l, p}
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
			return nil, err
		}

	}
	return b, nil

}

func (r *GwRebuild) exe() error {

	app := os.Getenv("GWCLI")
	configini := os.Getenv("INICONFIG")
	xmlconfig := os.Getenv("XMLCONFIG")

	tconfigini := fmt.Sprintf("%s/%s/%s", INPUT, r.Lastpath, CONFIGINI)
	txmlconfig := fmt.Sprintf("%s/%s/%s", INPUT, r.Lastpath, XMLCONFIG)

	cmd := exec.Command("cp", configini, tconfigini)
	err := cmd.Run()
	if err != nil {
		return err
	}

	cmd = exec.Command("cp", xmlconfig, txmlconfig)
	err = cmd.Run()
	if err != nil {
		return err
	}

	iniconf(tconfigini, r.Lastpath)

	args := fmt.Sprintf("%s -config=%s -xmlconfig=%s", app, tconfigini, txmlconfig)

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

func iniconf(p, randpath string) error {
	cfg, err := ini.Load(p)
	if err != nil {
		return fmt.Errorf("Fail to read ini file  %s", err)
	}

	sec := cfg.Section(SECTION)
	err = inikey(sec, INPUTKEY, randpath)
	if err != nil {
		return err
	}
	err = inikey(sec, OUTPUTKEY, randpath)
	if err != nil {
		return err
	}
	err = cfg.SaveTo(p)
	if err != nil {
		return fmt.Errorf("Fail to save ini file : %s", err)

	}
	return nil

}
func inikey(s *ini.Section, keyname, randpath string) error {
	ok := s.HasKey(keyname)
	if !ok {
		return fmt.Errorf("Fail to find %s key", keyname)
	}
	key := s.Key(keyname)
	v := key.String()
	v = filepath.Join(v, randpath)
	key.SetValue(v)
	return nil

}
