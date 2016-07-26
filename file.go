package pdfhandler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PDF struct {
	FileName string            `json:"filename"`
	Fields   map[string]string `json:"fields"`
	Content  string            `json:"content"`
}

func (p PDF) render(rootPath string) ([]byte, error) {

	if p.Content != "" {
		return p.decodeContent()
	}

	if p.FileName == "" {
		return nil, errors.New("Invalid filename")
	}
	path := filepath.Join(rootPath, p.FileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	tmpfile, err := ioutil.TempFile("", "example")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write(mapToXFDF(p.Fields)); err != nil {
		return nil, err
	}
	if err := tmpfile.Close(); err != nil {
		return nil, err
	}
	cmd := exec.Command("pdftk", path, "fill_form", tmpfile.Name(), "output", "-")
	logger.Debugf("Executing pdftk %q", strings.Join(cmd.Args, " "))
	var out bytes.Buffer
	cmd.Stdout = &out
	var t bytes.Buffer
	cmd.Stderr = &t
	err = cmd.Run()
	if err != nil {
		return nil, errors.New(t.String())
	}
	return out.Bytes(), nil
}

func (p PDF) decodeContent() ([]byte, error) {
	return base64.StdEncoding.DecodeString(p.Content)
}
