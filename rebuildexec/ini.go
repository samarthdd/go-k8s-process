package rebuildexec

import (
	"fmt"
	"path/filepath"

	"github.com/go-ini/ini"
)

func iniconf(p, workDir string) error {
	cfg, err := ini.Load(p)
	if err != nil {
		return fmt.Errorf("Fail to read ini file  %s", err)
	}

	sec := cfg.Section(SECTION)
	err = inikey(sec, INPUTKEY, workDir, REBUILDINPUT)
	if err != nil {
		return err
	}
	err = inikey(sec, OUTPUTKEY, workDir, REBUILDOUTPUT)
	if err != nil {
		return err
	}
	err = cfg.SaveTo(p)
	if err != nil {
		return fmt.Errorf("Fail to save ini file : %s", err)

	}
	return nil

}
func inikey(s *ini.Section, keyname, workDir, ext string) error {
	ok := s.HasKey(keyname)
	if !ok {
		return fmt.Errorf("Fail to find %s key", keyname)
	}
	key := s.Key(keyname)
	v := key.String()
	v = filepath.Join(workDir, ext)
	key.SetValue(v)
	return nil

}
