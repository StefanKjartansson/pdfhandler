package pdfhandler

import (
	"bytes"
	"fmt"
)

func mapToXFDF(m map[string]string) []byte {
	buffer := bytes.NewBufferString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buffer.WriteString("<xfdf xmlns=\"http://ns.adobe.com/xfdf/\" xml:space=\"preserve\">")
	buffer.WriteString("<fields>")
	for k, v := range m {
		buffer.WriteString(fmt.Sprintf("<field name=\"%s\"><value>%s</value></field>\n", k, v))
	}
	buffer.WriteString("</fields>")
	buffer.WriteString("</xfdf>")
	return buffer.Bytes()
}
