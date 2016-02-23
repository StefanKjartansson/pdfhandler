# pdfhandler

[![Build Status](https://travis-ci.org/StefanKjartansson/pdfhandler.png?branch=master)](https://travis-ci.org/StefanKjartansson/pdfhandler)
[![Report Card](https://goreportcard.com/badge/github.com/StefanKjartansson/pdfhandler)](https://goreportcard.com/badge/github.com/StefanKjartansson/pdfhandler)

Simple `http.Handler` wrapping [pdftk](https://www.pdflabs.com/tools/pdftk-the-pdf-toolkit/) for filling form fields in pdfs. 

## Features

##### `GET` 

Lists pdf files in the pdf path and their fields.

```json
[
	{
		"filename": "myfile1.pdf",
		"fields": {"myfield": ""}
	}, 
	{
		"filename": "myfile2.pdf",
		"fields": {"other_field": ""}
	}
]
```

##### `POST`

Accepts either a json body `{"filename": "file", "fields": {"fieldName": "field"}}` of a single file or a json body list with the same structure. If a list is received and the `Accept` header is set to `application/pdf` the server returns a concatenated pdf. If the `Accept` header is set to `application/zip` the server returns a zip file containing the filled pdfs.

## Installing

```bash
go get github.com/StefanKjartansson/pdfhandler
```

### Usage

```go
// main.go
package main

import (
  "os"	
  "net/http"
  "log"
  "github.com/gorilla/mux"
)

func main() {
	pdfFilePath := os.Getenv("PDF_PATH")
	if pdfFilePath == "" {
		log.Fatal("PDF_PATH is required")
	}
	router := mux.NewRouter()
	pdfHandler, err := pdfhandler.New(pdfFilePath)
	if err != nil {
		log.Fatal(err)
	}
	router.Handle("/pdf/", pdfHandler)
	http.Handle("/", router)
  	log.Fatal(http.ListenAndServe(":3001", nil))
}
```

#### Usage example

Fill in the fields of `myfile1.pdf` & `myfile2.pdf`, return a concatenated pdf.

```bash
curl \
    -H "Content-Type: application/json" \
    -H "Accept: application/pdf" \
    -H "X-Filename: myfile" \
    -X POST \
    -d '[{"filename": "myfile1.pdf","fields": {"myfield": "hello"}}, {"filename": "myfile2.pdf","fields": {"other_field": "world"}}]' \
    http://127.0.0.1:3001/pdf/
```
