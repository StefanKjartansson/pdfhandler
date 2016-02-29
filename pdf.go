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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

var (
	acceptedContentTypes = []string{
		"application/zip",
		"application/pdf",
	}
	logger StdLogger
)

func init() {
	logger = &noOpLogger{}
}

type StdLogger interface {
	Debugf(string, ...interface{})
	Errorf(string, ...interface{})
}

type noOpLogger struct{}

func (l *noOpLogger) Debugf(format string, args ...interface{}) {
}
func (l *noOpLogger) Errorf(format string, args ...interface{}) {
}

func Error(w http.ResponseWriter, error string, code int) {
	logger.Errorf("http error: %q", error)
	http.Error(w, error, code)
}

func SetLogger(l StdLogger) {
	logger = l
}

type PDFHandler struct {
	filePath string
}

func New(path string) (*PDFHandler, error) {
	_, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	return &PDFHandler{path}, nil
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
				logger.Debugf("Rendering %s/%s to %s", ph.filePath, p.FileName, tmpfn)
				b, err := p.render(ph.filePath)
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
		files := []string{}
		for j := range ch {
			files = append(files, j.File)
		}
		sort.Strings(files)
		for _, f := range files {
			cmd.Args = append(cmd.Args, f)
		}
		cmd.Args = append(cmd.Args, "cat")
		cmd.Args = append(cmd.Args, "output")
		cmd.Args = append(cmd.Args, "-")
		logger.Debugf("Executing pdftk: %q", strings.Join(cmd.Args, " "))
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
			header := &zip.FileHeader{
				Name:   j.Pdf.FileName,
				Method: zip.Deflate,
			}
			header.SetModTime(time.Now())
			f, err := zw.CreateHeader(header)
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
	files, err := ioutil.ReadDir(p.filePath)
	if err != nil {
		Error(w, err.Error(), http.StatusInternalServerError)
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
				p, err := readFields(p.filePath, fp)
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
		Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (p PDFHandler) post(w http.ResponseWriter, req *http.Request) {
	ct := req.Header.Get("Content-Type")
	if ct != "application/json" {
		Error(w, "Invalid Content-Type", http.StatusBadRequest)
		return
	}

	ac := req.Header.Get("Accept")
	if !stringInSlice(ac, acceptedContentTypes) {
		Error(w, "Invalid Accept header", http.StatusBadRequest)
		return
	}

	filename := req.Header.Get("X-Filename")
	if filename == "" {
		filename = uuid.NewV4().String()
	}
	if strings.HasSuffix(ac, "zip") {
		filename += ".zip"
	} else if strings.HasSuffix(ac, "pdf") {
		filename += ".pdf"
	}

	r := bufio.NewReader(req.Body)
	dec := json.NewDecoder(r)
	ch, _ := r.Peek(1)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", ac)

	switch string(ch) {
	case "{":
		var x PDF
		err := dec.Decode(&x)
		if err != nil {
			Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out, err := x.render(p.filePath)
		if err != nil {
			Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(out)
		break
	case "[":
		var pdfs []PDF
		err := dec.Decode(&pdfs)
		if err != nil {
			Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = p.multi(ac, pdfs, w)
		if err != nil {
			Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		break
	default:
		Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
}

func (p PDFHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		p.get(w, req)
	case "POST":
		p.post(w, req)
	default:
		Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}
