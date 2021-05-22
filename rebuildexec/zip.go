package rebuildexec

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
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
		rc, err := f.Open()
		if err != nil {
			return err
		}
		zh := ziphelper{
			b:        nil,
			fullName: f.Name,
			name:     filepath.Base(f.Name),
		}
		z.zipEntity = append(z.zipEntity, zh)

		buf, err := ioutil.ReadAll(rc)
		if err != nil {
			return err

		}

		err = ioutil.WriteFile(filepath.Join(z.workdir, zh.name), buf, 0666)
		if err != nil {
			return err
		}

		rc.Close()
	}
	return nil
}

func (z *zipProcess) openAllFilesExt(extFolder string) error {
	var err error
	for _, v := range z.zipEntity {
		p := filepath.Join(z.workdir, extFolder, v.name)
		fp := fmt.Sprintf("%s%s", p, z.ext)
		v.b, err = ioutil.ReadFile(fp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (z *zipProcess) writeZip(zipFileName string) {
	buf := new(bytes.Buffer)

	// Create a new zip archive.
	w := zip.NewWriter(buf)

	// Add some files to the archive.

	for _, zh := range z.zipEntity {

		f, err := w.Create(zh.fullName)
		if err != nil {
			log.Fatal(err)
		}
		_, err = f.Write(zh.b)
		if err != nil {
			log.Fatal(err)
		}
	}
	err := w.Close()
	if err != nil {
		log.Fatal(err)
	}
	b, err := ioutil.ReadAll(buf)

	zipFilePath := filepath.Join(z.workdir, zipFileName)
	os.WriteFile(zipFilePath, b, 0777)

}

func (r *GwRebuild) zipRebuildFiles() {

}

func (r *GwRebuild) zipReports() {

}

func (r *GwRebuild) zipLogs() {
}

//copy zip file to input
//extract zip files to input with ilepath.Base
//rebuild files
//zip rebuilt files with the full path
//zip report files
//zip log files
