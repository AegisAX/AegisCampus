package mailer

import (
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"path/filepath"
	"strings"
	"time"
)

// WriteTo implements io.WriterTo. It dumps the whole message into w.
func (m *Message) WriteTo(w io.Writer) (int64, error) {
	mw := &messageWriter{w: w}
	mw.writeMessage(m)
	return mw.n, mw.err
}

func (w *messageWriter) writeMessage(m *Message) {
	if _, ok := m.header["Mime-Version"]; !ok {
		w.writeString("Mime-Version: 1.0\r\n")
	}
	if _, ok := m.header["Date"]; !ok {
		w.writeHeader("Date", m.FormatDate(now()))
	}
	// Emit top-level headers in insertion order so that the output is
	// deterministic and matches the order SetHeader was called.
	// Bcc is intentionally omitted from the serialized output.
	for _, k := range m.headerKeys {
		if k == "Bcc" {
			continue
		}
		if v, ok := m.header[k]; ok {
			w.writeHeader(k, v...)
		}
	}

	if m.hasMixedPart() {
		w.openMultipart("mixed")
	}
	if m.hasRelatedPart() {
		w.openMultipart("related")
	}
	if m.hasAlternativePart() {
		w.openMultipart("alternative")
	}
	for _, part := range m.parts {
		w.writePart(part, m.charset)
	}
	if m.hasAlternativePart() {
		w.closeMultipart()
	}

	w.addFiles(m.embedded, false)
	if m.hasRelatedPart() {
		w.closeMultipart()
	}

	w.addFiles(m.attachments, true)
	if m.hasMixedPart() {
		w.closeMultipart()
	}
}

func (m *Message) hasMixedPart() bool {
	return (len(m.parts) > 0 && len(m.attachments) > 0) || len(m.attachments) > 1
}

func (m *Message) hasRelatedPart() bool {
	return (len(m.parts) > 0 && len(m.embedded) > 0) || len(m.embedded) > 1
}

func (m *Message) hasAlternativePart() bool {
	return len(m.parts) > 1
}

type messageWriter struct {
	w          io.Writer
	n          int64
	writers    [3]*multipart.Writer
	partWriter io.Writer
	depth      uint8
	err        error
}

func (w *messageWriter) openMultipart(mimeType string) {
	mw := multipart.NewWriter(w)
	contentType := "multipart/" + mimeType + ";\r\n boundary=" + mw.Boundary()
	w.writers[w.depth] = mw

	if w.depth == 0 {
		w.writeHeader("Content-Type", contentType)
		w.writeString("\r\n")
	} else {
		w.createPart(map[string][]string{
			"Content-Type": {contentType},
		})
	}
	w.depth++
}

func (w *messageWriter) createPart(h map[string][]string) {
	w.partWriter, w.err = w.writers[w.depth-1].CreatePart(h)
}

func (w *messageWriter) closeMultipart() {
	if w.depth > 0 {
		w.writers[w.depth-1].Close()
		w.depth--
	}
}

func (w *messageWriter) writePart(p *part, charset string) {
	w.writeHeaders(map[string][]string{
		"Content-Type":              {p.contentType + "; charset=" + charset},
		"Content-Transfer-Encoding": {string(p.encoding)},
	})
	w.writeBody(p.copier, p.encoding)
}

func (w *messageWriter) addFiles(files []*file, isAttachment bool) {
	for _, f := range files {
		if _, ok := f.Header["Content-Type"]; !ok {
			mediaType := mime.TypeByExtension(filepath.Ext(f.Name))
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
			f.setHeader("Content-Type", mediaType+`; name="`+f.Name+`"`)
		}
		if _, ok := f.Header["Content-Transfer-Encoding"]; !ok {
			f.setHeader("Content-Transfer-Encoding", string(Base64))
		}
		if _, ok := f.Header["Content-Disposition"]; !ok {
			disp := "inline"
			if isAttachment {
				disp = "attachment"
			}
			f.setHeader("Content-Disposition", disp+`; filename="`+f.Name+`"`)
		}
		if !isAttachment {
			if _, ok := f.Header["Content-ID"]; !ok {
				f.setHeader("Content-ID", "<"+f.Name+">")
			}
		}
		w.writeHeaders(f.Header)
		w.writeBody(f.CopyFunc, Base64)
	}
}

// setHeader is a small helper used by addFiles to populate default file headers.
func (f *file) setHeader(field, value string) {
	f.Header[field] = []string{value}
}

func (w *messageWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, errors.New("mailer: cannot write as writer is in error")
	}
	var n int
	n, w.err = w.w.Write(p)
	w.n += int64(n)
	return n, w.err
}

func (w *messageWriter) writeString(s string) {
	n, _ := io.WriteString(w.w, s)
	w.n += int64(n)
}

func (w *messageWriter) writeHeader(k string, v ...string) {
	w.writeString(k)
	if len(v) == 0 {
		w.writeString(":\r\n")
		return
	}
	w.writeString(": ")

	// RFC 5322 78-char + RFC 2047 76-char limit. Use 76 for simplicity.
	charsLeft := 76 - len(k) - len(": ")
	for i, s := range v {
		if charsLeft < 1 {
			if i == 0 {
				w.writeString("\r\n ")
			} else {
				w.writeString(",\r\n ")
			}
			charsLeft = 75
		} else if i != 0 {
			w.writeString(", ")
			charsLeft -= 2
		}
		for len(s) > charsLeft {
			s = w.writeLine(s, charsLeft)
			charsLeft = 75
		}
		w.writeString(s)
		if i := lastIndexByte(s, '\n'); i != -1 {
			charsLeft = 75 - (len(s) - i - 1)
		} else {
			charsLeft -= len(s)
		}
	}
	w.writeString("\r\n")
}

func (w *messageWriter) writeLine(s string, charsLeft int) string {
	if i := strings.IndexByte(s, '\n'); i != -1 && i < charsLeft {
		w.writeString(s[:i+1])
		return s[i+1:]
	}
	for i := charsLeft - 1; i >= 0; i-- {
		if s[i] == ' ' {
			w.writeString(s[:i])
			w.writeString("\r\n ")
			return s[i+1:]
		}
	}
	for i := 75; i < len(s); i++ {
		if s[i] == ' ' {
			w.writeString(s[:i])
			w.writeString("\r\n ")
			return s[i+1:]
		}
		if s[i] == '\n' {
			w.writeString(s[:i+1])
			return s[i+1:]
		}
	}
	w.writeString(s)
	return ""
}

// writeHeaders emits headers in one of two modes:
//
//   - depth == 0: write headers directly into the current header section.
//     Used by openMultipart when it pushes a multipart Content-Type header
//     at the top level. Bcc is intentionally omitted.
//
//   - depth > 0: delegate to createPart, which writes a part boundary and
//     emits headers for the new nested MIME part.
//
// Note: writeMessage emits top-level message headers in insertion order
// via m.headerKeys, not through this function.
func (w *messageWriter) writeHeaders(h map[string][]string) {
	if w.depth == 0 {
		for k, v := range h {
			if k != "Bcc" {
				w.writeHeader(k, v...)
			}
		}
	} else {
		w.createPart(h)
	}
}

func (w *messageWriter) writeBody(f func(io.Writer) error, enc Encoding) {
	var subWriter io.Writer
	if w.depth == 0 {
		w.writeString("\r\n")
		subWriter = w.w
	} else {
		subWriter = w.partWriter
	}

	switch enc {
	case Base64:
		wc := base64.NewEncoder(base64.StdEncoding, newBase64LineWriter(subWriter))
		w.err = f(wc)
		wc.Close()
	case Unencoded:
		w.err = f(subWriter)
	default: // QuotedPrintable
		wc := newQPWriter(subWriter)
		w.err = f(wc)
		wc.Close()
	}
}

// RFC 2045 6.7 (quoted-printable) and 6.8 (base64) require <=76 chars per line.
const maxLineLen = 76

// base64LineWriter wraps base64 output to enforce the 76-char line limit.
type base64LineWriter struct {
	w       io.Writer
	lineLen int
}

func newBase64LineWriter(w io.Writer) *base64LineWriter {
	return &base64LineWriter{w: w}
}

func (w *base64LineWriter) Write(p []byte) (int, error) {
	n := 0
	for len(p)+w.lineLen > maxLineLen {
		w.w.Write(p[:maxLineLen-w.lineLen])
		w.w.Write([]byte("\r\n"))
		p = p[maxLineLen-w.lineLen:]
		n += maxLineLen - w.lineLen
		w.lineLen = 0
	}
	w.w.Write(p)
	w.lineLen += len(p)
	return n + len(p), nil
}

// Stubbed out for testing.
var now = time.Now

// === MIME serialization helpers ===
// Sentinel requires Go 1.23+ so the legacy gopkg.in/alexcesaro fallback is unnecessary.

var newQPWriter = quotedprintable.NewWriter
var lastIndexByte = strings.LastIndexByte
