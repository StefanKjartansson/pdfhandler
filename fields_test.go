package pdfhandler

import (
	"strings"
	"testing"
)

func TestMapToXFDF(t *testing.T) {
	in := map[string]string{
		"Foo": "Bar",
		"Bar": "Baz",
	}
	out := string(mapToXFDF(in))
	expected := `<field name="Foo"><value>Bar</value></field>`
	if !strings.Contains(out, expected) {
		t.Fatalf("Expected %q to contain %s", out, expected)
	}
}
