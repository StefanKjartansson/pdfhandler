package pdfhandler

import (
	"archive/zip"
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

var (
	acceptedContentTypes = []string{
		"application/zip",
		"application/pdf",
	}
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

func (ph PDFHandler) multi(mimetype string, pdfs []PDF, w http.ResponseWriter) error {
	dir, err := ioutil.TempDir("", "workpath")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	type job struct {
		Pdf  PDF
		File string
		Body []byte
	}
	ch := make(chan job)

	go func() {
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
				ch <- job{p, tmpfn, b}
			}(idx, p)
		}
		wg.Wait()
		close(ch)
	}()

	switch mimetype {
	case "application/pdf":
		cmd := exec.Command("pdftk")
		for j := range ch {
			cmd.Args = append(cmd.Args, j.File)
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
		break
	case "application/zip":
		zw := zip.NewWriter(w)
		for j := range ch {
			f, err := zw.Create(j.Pdf.FileName)
			if err != nil {
				return err
			}
			_, err = f.Write(j.Body)
			if err != nil {
				return err
			}
		}
		return zw.Close()
		break
	}
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
	ct := req.Header.Get("Content-Type")
	if ct != "application/json" {
		http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}

	ac := req.Header.Get("Accept")
	if !stringInSlice(ac, acceptedContentTypes) {
		http.Error(w, "Invalid Accept header", http.StatusBadRequest)
		return
	}

	r := bufio.NewReader(req.Body)
	dec := json.NewDecoder(r)
	ch, _ := r.Peek(1)
	switch string(ch) {
	case "{":
		var x PDF
		err := dec.Decode(&x)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
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
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = p.multi(ac, pdfs, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		break
	default:
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=file.pdf")
	w.Header().Set("Content-Type", ac)
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
