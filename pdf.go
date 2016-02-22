package pdfhandler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type PDF struct {
	FileName string            `json:"filename"`
	Fields   map[string]string `json:"fields"`
}

func (p PDF) render(rootPath string) ([]byte, error) {
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

type PDFHandler struct {
	FilePath string
}

func (ph PDFHandler) multi(pdfs []PDF, w http.ResponseWriter) error {
	dir, err := ioutil.TempDir("", "workpath")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	var wg sync.WaitGroup
	for idx, p := range pdfs {
		wg.Add(1)
		go func(idx int, p PDF) {
			defer wg.Done()
			tmpfn := filepath.Join(dir, fmt.Sprintf("%d.pdf", idx))
			b, err := p.render(ph.FilePath)
			if err != nil {
				return
			}
			ioutil.WriteFile(tmpfn, b, 0777)
		}(idx, p)
	}
	wg.Wait()

	cmd := exec.Command("pdftk")
	for idx := range pdfs {
		cmd.Args = append(cmd.Args, filepath.Join(dir, fmt.Sprintf("%d.pdf", idx)))
	}
	cmd.Args = append(cmd.Args, "cat")
	cmd.Args = append(cmd.Args, "output")
	cmd.Args = append(cmd.Args, "-")
	var out bytes.Buffer
	cmd.Stdout = &out
	var t bytes.Buffer
	cmd.Stderr = &t
	err = cmd.Run()
	if err != nil {
		return errors.New(t.String())
	}
	w.Write(out.Bytes())
	return nil
}

func (p PDFHandler) get(w http.ResponseWriter, req *http.Request) {
	files, err := ioutil.ReadDir(p.FilePath)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	ch := make(chan PDF)

	go func() {
		var wg sync.WaitGroup
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".pdf") {
				continue
			}
			wg.Add(1)
			go func(fp string) {
				p, err := readFields(p.FilePath, fp)
				if err == nil {
					ch <- *p
				}
				defer wg.Done()
			}(file.Name())
		}
		wg.Wait()
		close(ch)
	}()

	pdfs := []PDF{}
	for p := range ch {
		pdfs = append(pdfs, p)
	}

	enc := json.NewEncoder(w)
	err = enc.Encode(pdfs)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

}

func (p PDFHandler) post(w http.ResponseWriter, req *http.Request) {
	r := bufio.NewReader(req.Body)
	dec := json.NewDecoder(r)
	ch, _ := r.Peek(1)
	switch string(ch) {
	case "{":
		var x PDF
		err := dec.Decode(&x)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
		out, err := x.render(p.FilePath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(out)
		break
	case "[":
		var pdfs []PDF
		err := dec.Decode(&pdfs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotAcceptable)
			return
		}
		err = p.multi(pdfs, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		break
	default:
		http.Error(w, "Invalid input", http.StatusNotAcceptable)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=file.pdf")
	w.Header().Set("Content-Type", "application/pdf")
}

func (p PDFHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		p.get(w, req)
	case "POST":
		p.post(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}
