package rebuildexec

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
)

type zipProcess struct {
	workdir   string
	zipEntity []ziphelper
	ext       string
}
type ziphelper struct {
	b        []byte
	fullName string
	name     string
	RandName string
}

func (z *zipProcess) openZip(path string) error {
	fpath := filepath.Join(z.workdir, path)
	r, err := zip.OpenReader(fpath)

	if err != nil {
		return err
	}

	defer r.Close()

	// Iterate through the files in the archive,
	// printing some of their contents.
	for _, f := range r.File {
		randStr := RandStringRunes(16)

		rc, err := f.Open()
		if err != nil {
			return err
		}
		fName := f.Name
		if fName[len(fName)-1] == '/' {
			rc.Close()
			continue
		}

		zh := ziphelper{
			b:        nil,
			fullName: fName,
			name:     filepath.Base(fName),
			RandName: randStr,
		}
		z.zipEntity = append(z.zipEntity, zh)

		buf, err := ioutil.ReadAll(rc)
		if err != nil {
			return err

		}

		err = ioutil.WriteFile(filepath.Join(z.workdir, zh.RandName), buf, 0666)
		if err != nil {
			return err
		}

		rc.Close()
	}
	return nil
}

func (z *zipProcess) readAllFilesExt(extFolder string) {
	var err error
	for i := 0; i < len(z.zipEntity); i++ {
		p := filepath.Join(z.workdir, extFolder, z.zipEntity[i].RandName)
		fp := fmt.Sprintf("%s%s", p, z.ext)
		z.zipEntity[i].b, err = ioutil.ReadFile(fp)
		if err != nil {
			log.Println(err)
		}
	}

}

func (z *zipProcess) writeZip(zipFileName string) error {
	empty := true

	ext := z.ext
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add some files to the archive.

	for _, zh := range z.zipEntity {
		if zh.b == nil {
			continue
		}
		empty = false
		fName := zh.fullName
		if ext != "" {
			fName = fmt.Sprintf("%s%s", zh.name, ext)
		}

		h := zip.FileHeader{
			Name: fName,

			Modified: time.Now(),
		}

		f, err := w.CreateHeader(&h)

		if err != nil {
			return err
		}
		_, err = f.Write(zh.b)
		if err != nil {
			return err
		}
	}
	err := w.Close()
	if err != nil {
		return err
	}

	if empty {
		return fmt.Errorf("there is no log files ")
	}

	b, err := ioutil.ReadAll(buf)
	if err != nil {
		return err
	}
	zipFilePath := filepath.Join(z.workdir, zipFileName)
	err = os.WriteFile(zipFilePath, b, 0777)
	return err

}
