package pdfhandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ts *httptest.Server
var (
	single = PDF{
		"OoPdfFormExample.pdf",
		map[string]string{"Family Name Text Box": "Barsson"},
	}
	multi = []PDF{
		{
			"OoPdfFormExample.pdf",
			map[string]string{"Family Name Text Box": "Barsson"},
		},
		{
			"OoPdfFormExample.pdf",
			map[string]string{"Family Name Text Box": "Barsson"},
		},
	}
)

func TestMain(m *testing.M) {
	pdfHandler, _ := New("./pdf-test")
	ts = httptest.NewServer(pdfHandler)
	defer ts.Close()
	os.Exit(m.Run())
}

func TestPDFStruct(t *testing.T) {
	_, err := single.render("./pdf-test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGet(t *testing.T) {
	req, err := http.NewRequest("GET", ts.URL, nil)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestPostSingle(t *testing.T) {
	b, err := json.Marshal(single)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBuffer(b))
	req.Header.Set("Accept", "application/pdf")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestPostMulti(t *testing.T) {
	b, err := json.Marshal(multi)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBuffer(b))
	req.Header.Set("Accept", "application/pdf")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		t.Fatalf("Got status %d and body %q", resp.StatusCode, buf.String())
	}
	ct := resp.Header.Get("Content-Type")
	assert.Equal(t, ct, "application/pdf")
}

func TestPostMultiFilename(t *testing.T) {
	b, err := json.Marshal(multi)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBuffer(b))
	req.Header.Set("Accept", "application/pdf")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Filename", "myfile")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		t.Fatalf("Got status %d and body %q", resp.StatusCode, buf.String())
	}
	t.Logf("%q", resp.Header)
	ct := resp.Header.Get("Content-Type")
	assert.Equal(t, ct, "application/pdf")
	cd := resp.Header.Get("Content-Disposition")
	assert.Equal(t, cd, fmt.Sprintf("attachment; filename=%s", "myfile.pdf"))
}

func TestPostMultiZip(t *testing.T) {
	b, err := json.Marshal(multi)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBuffer(b))
	req.Header.Set("Accept", "application/zip")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal("Not working")
	}
	ct := resp.Header.Get("Content-Type")
	assert.Equal(t, ct, "application/zip")
}

func TestInvalidContentType(t *testing.T) {
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "something else")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestInvalidAccept(t *testing.T) {
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "something else")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestInvalidBody(t *testing.T) {
	req, err := http.NewRequest("POST", ts.URL, bytes.NewBufferString(""))
	req.Header.Set("Accept", "application/zip")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestInvalidMethod(t *testing.T) {
	req, err := http.NewRequest("DELETE", ts.URL, bytes.NewBufferString(""))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, resp.StatusCode, http.StatusMethodNotAllowed)
}
