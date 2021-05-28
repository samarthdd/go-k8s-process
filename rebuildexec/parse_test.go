package rebuildexec

import (
	"testing"
)

func TestParseContnetType(t *testing.T) {
	nonCompatible := "pdf"
	validPdf := "application/pdf"
	validPng := "image/png"
	ctTest := []struct {
		ct     string
		result string
	}{
		{nonCompatible, "pdf"},
		{validPng, "png"},
		{validPdf, "pdf"},
	}
	for _, v := range ctTest {
		res := parseContnetType(v.ct)
		if res != v.result {
			t.Errorf("fails expected %s got %s", v.result, res)
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
