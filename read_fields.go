package pdfhandler

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
)

func scanFields(filename string, r io.Reader) *PDF {
	p := PDF{
		FileName: filename,
		Fields:   make(map[string]string),
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "FieldName:") {
			p.Fields[strings.TrimSpace(t[11:])] = ""
		}
	}
	return &p
}

func readFields(rootPath, fp string) (*PDF, error) {
	path := filepath.Join(rootPath, fp)
	cmd := exec.Command("pdftk", path, "dump_data_fields_utf8")
	var out bytes.Buffer
	cmd.Stdout = &out
	var t bytes.Buffer
	cmd.Stderr = &t
	err := cmd.Run()
	if err != nil {
		return nil, errors.New(t.String())
	}
	return scanFields(fp, &out), nil
}
