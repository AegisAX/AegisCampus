package mailer

import (
	"io"
	"mime"
	"strings"
	"time"

	"os"
	"path/filepath"

	"github.com/AegisAX/AegisCampus/util/mimeutil"
)

// Encoding represents a MIME body encoding scheme.
type Encoding string

const (
	QuotedPrintable Encoding = "quoted-printable"
	Base64          Encoding = "base64"
	Unencoded       Encoding = "8bit"
)

// Message represents an email message — a 1:1 compatible replacement
// the previous external mailer used by AegisCampus.
type Message struct {
	header      map[string][]string
	headerKeys  []string
	parts       []*part
	attachments []*file
	embedded    []*file
	charset     string
	encoding    Encoding
}

type part struct {
	contentType string
	copier      func(io.Writer) error
	encoding    Encoding
}

type file struct {
	Name     string
	Header   map[string][]string
	CopyFunc func(w io.Writer) error
}

// MessageSetting configures a Message at construction time.
type MessageSetting func(m *Message)

// SetCharset overrides the default UTF-8 charset.
func SetCharset(charset string) MessageSetting {
	return func(m *Message) { m.charset = charset }
}

// SetEncoding overrides the default quoted-printable body encoding.
func SetEncoding(enc Encoding) MessageSetting {
	return func(m *Message) { m.encoding = enc }
}

// NewMessage creates a new message with UTF-8 + quoted-printable defaults.
func NewMessage(settings ...MessageSetting) *Message {
	m := &Message{
		header:   make(map[string][]string),
		charset:  "UTF-8",
		encoding: QuotedPrintable,
	}
	for _, s := range settings {
		s(m)
	}
	return m
}

// encodeString RFC 2047 Q-encodes a header value if needed (ASCII pass-through).
func (m *Message) encodeString(value string) string {
	if m.encoding == Base64 {
		return mimeutil.EncodeHeaderRFC2047B(value)
	}
	return mimeutil.EncodeHeaderRFC2047(value)
}

// SetHeader sets a header field to one or more encoded values.
func (m *Message) SetHeader(field string, value ...string) {
	for i := range value {
		value[i] = m.encodeString(value[i])
	}
	m.trackHeaderKey(field)
	m.header[field] = value
}

// SetHeaders sets multiple headers at once.
func (m *Message) SetHeaders(h map[string][]string) {
	for k, v := range h {
		m.SetHeader(k, v...)
	}
}

// GetHeader returns the raw stored values for a header field.
func (m *Message) GetHeader(field string) []string {
	return m.header[field]
}

// SetAddressHeader sets a field to an RFC 5322 address with optional display name.
func (m *Message) SetAddressHeader(field, address, name string) {
	m.trackHeaderKey(field)
	m.header[field] = []string{m.FormatAddress(address, name)}
}

// FormatAddress formats an address with an optional display name.
// FormatAddress formats an RFC 5322 address with optional display name,
// applying RFC 2047 encoding when the name contains non-ASCII characters.
func (m *Message) FormatAddress(address, name string) string {
	if name == "" {
		return address
	}
	enc := m.encodeString(name)
	var quoted string
	if enc == name {
		// ASCII: quote the name with backslash-escapes
		var b strings.Builder
		b.WriteByte('"')
		for i := 0; i < len(name); i++ {
			c := name[i]
			if c == '\\' || c == '"' {
				b.WriteByte('\\')
			}
			b.WriteByte(c)
		}
		b.WriteByte('"')
		quoted = b.String()
	} else if hasSpecials(name) {
		quoted = mime.BEncoding.Encode(m.charset, name)
	} else {
		quoted = enc
	}
	return quoted + " <" + address + ">"
}

// hasSpecials reports whether s contains any RFC 5322 special character.
func hasSpecials(text string) bool {
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(', ')', '<', '>', '[', ']', ':', ';', '@', '\\', ',', '.', '"':
			return true
		}
	}
	return false
}

// SetDateHeader sets a date-valued header in RFC 1123Z format.
func (m *Message) SetDateHeader(field string, date time.Time) {
	m.trackHeaderKey(field)
	m.header[field] = []string{m.FormatDate(date)}
}

// FormatDate formats a time as RFC 1123Z (RFC 5322 compliant).
func (m *Message) FormatDate(date time.Time) string {
	return date.Format(time.RFC1123Z)
}

// trackHeaderKey appends field to headerKeys if not already present.
// Setting the same key twice keeps its original insertion position so that
// the serialized output stays deterministic.
func (m *Message) trackHeaderKey(field string) {
	for _, k := range m.headerKeys {
		if k == field {
			return
		}
	}
	m.headerKeys = append(m.headerKeys, field)
}

// Reset clears all message state but preserves charset/encoding settings.
func (m *Message) Reset() {
	for k := range m.header {
		delete(m.header, k)
	}
	m.headerKeys = nil
	m.parts = nil
	m.attachments = nil
	m.embedded = nil
}

// SetBody sets the body of the message. Replaces any previous SetBody or AddAlternative.
func (m *Message) SetBody(contentType, body string, settings ...PartSetting) {
	m.parts = []*part{m.newPart(contentType, newCopier(body), settings)}
}

// AddAlternative adds an alternative part to the message. Commonly used to send
// HTML emails with a plain-text fallback. Add plain text before HTML.
func (m *Message) AddAlternative(contentType, body string, settings ...PartSetting) {
	m.AddAlternativeWriter(contentType, newCopier(body), settings...)
}

// AddAlternativeWriter adds an alternative part using a writer callback.
func (m *Message) AddAlternativeWriter(contentType string, f func(io.Writer) error, settings ...PartSetting) {
	m.parts = append(m.parts, m.newPart(contentType, f, settings))
}

func (m *Message) newPart(contentType string, f func(io.Writer) error, settings []PartSetting) *part {
	p := &part{
		contentType: contentType,
		copier:      f,
		encoding:    m.encoding,
	}
	for _, s := range settings {
		s(p)
	}
	return p
}

func newCopier(s string) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := io.WriteString(w, s)
		return err
	}
}

// PartSetting configures a body part added via SetBody / AddAlternative / AddAlternativeWriter.
type PartSetting func(*part)

// SetPartEncoding overrides the part-level body encoding.
func SetPartEncoding(e Encoding) PartSetting {
	return PartSetting(func(p *part) {
		p.encoding = e
	})
}

// FileSetting configures an Attach or Embed call.
type FileSetting func(*file)

// SetHeader is a FileSetting to set the MIME header of the message part that
// contains the file content. Mandatory headers are added automatically at send
// time if not already set.
func SetHeader(h map[string][]string) FileSetting {
	return func(f *file) {
		for k, v := range h {
			f.Header[k] = v
		}
	}
}

// Rename overrides the attachment's display name (Name field).
func Rename(name string) FileSetting {
	return func(f *file) {
		f.Name = name
	}
}

// SetCopyFunc replaces the content writer used at send time.
// Default: read file content from disk via Attach(filename, ...).
func SetCopyFunc(f func(io.Writer) error) FileSetting {
	return func(fi *file) {
		fi.CopyFunc = f
	}
}

// Attach reads filename at send time and adds it as a multipart/mixed attachment.
func (m *Message) Attach(filename string, settings ...FileSetting) {
	m.attachments = m.appendFile(m.attachments, fileFromFilename(filename), settings)
}

// AttachReader uses an io.Reader for attachment content (consumed at send time).
func (m *Message) AttachReader(name string, r io.Reader, settings ...FileSetting) {
	m.attachments = m.appendFile(m.attachments, fileFromReader(name, r), settings)
}

// Embed reads filename at send time and adds it as a multipart/related inline image.
func (m *Message) Embed(filename string, settings ...FileSetting) {
	m.embedded = m.appendFile(m.embedded, fileFromFilename(filename), settings)
}

// EmbedReader uses an io.Reader for inline image content.
func (m *Message) EmbedReader(name string, r io.Reader, settings ...FileSetting) {
	m.embedded = m.appendFile(m.embedded, fileFromReader(name, r), settings)
}

func (m *Message) appendFile(list []*file, f *file, settings []FileSetting) []*file {
	for _, s := range settings {
		s(f)
	}
	if list == nil {
		return []*file{f}
	}
	return append(list, f)
}

func fileFromFilename(name string) *file {
	return &file{
		Name:   filepath.Base(name),
		Header: make(map[string][]string),
		CopyFunc: func(w io.Writer) error {
			h, err := os.Open(name)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, h); err != nil {
				h.Close()
				return err
			}
			return h.Close()
		},
	}
}

func fileFromReader(name string, r io.Reader) *file {
	return &file{
		Name:   filepath.Base(name),
		Header: make(map[string][]string),
		CopyFunc: func(w io.Writer) error {
			_, err := io.Copy(w, r)
			return err
		},
	}
}
