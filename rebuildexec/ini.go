package rebuildexec

import (
	"fmt"

	"github.com/go-ini/ini"
)

func inikey(s *ini.Section, keyname, keyvalue string) error {
	ok := s.HasKey(keyname)
	if !ok {
		return fmt.Errorf("fail to find %s key", keyname)
	}
	key := s.Key(keyname)
	key.SetValue(keyvalue)
	return nil

}
