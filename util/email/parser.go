package emailparser

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
)

// NewMessageFromReader parses a raw RFC 5322 message from r.
func NewMessageFromReader(r io.Reader) (*Message, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, err
	}

	m := &Message{
		Headers: make(textproto.MIMEHeader),
	}
	for k, v := range msg.Header {
		m.Headers[k] = v
	}

	m.From = decodeHeader(msg.Header.Get("From"))
	m.Subject = decodeHeader(msg.Header.Get("Subject"))
	m.To = parseAddressList(msg.Header.Get("To"))

	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return nil, err
	}

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// No or invalid Content-Type → default to text/plain
		mediaType = "text/plain"
		params = map[string]string{}
	}

	encoding := msg.Header.Get("Content-Transfer-Encoding")
	header := textproto.MIMEHeader(msg.Header)
	if err := m.processBody(body, mediaType, params, encoding, header); err != nil {
		return nil, err
	}

	return m, nil
}

// processBody dispatches to multipart or single-part handler.
func (m *Message) processBody(body []byte, mediaType string, params map[string]string, encoding string, header textproto.MIMEHeader) error {
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return errors.New("emailparser: multipart message missing boundary")
		}
		return m.processMultipart(body, boundary)
	}
	return m.processPart(body, mediaType, encoding, header)
}

// processMultipart iterates each MIME part. Nested multiparts recurse.
func (m *Message) processMultipart(body []byte, boundary string) error {
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		partBody, err := io.ReadAll(p)
		if err != nil {
			return err
		}
		partMediaType, partParams, perr := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if perr != nil {
			partMediaType = "text/plain"
			partParams = map[string]string{}
		}
		partEncoding := p.Header.Get("Content-Transfer-Encoding")
		if err := m.processBody(partBody, partMediaType, partParams, partEncoding, p.Header); err != nil {
			return err
		}
	}
}

// processPart handles a single MIME part: decode encoding, classify as Text/HTML/Attachment.
func (m *Message) processPart(body []byte, mediaType, encoding string, header textproto.MIMEHeader) error {
	decoded, err := decodeContent(body, encoding)
	if err != nil {
		return err
	}

	// Attachment detection: Content-Disposition: attachment OR a filename parameter
	disposition := header.Get("Content-Disposition")
	dispType, dispParams, _ := mime.ParseMediaType(disposition)
	filename := dispParams["filename"]
	if filename == "" {
		// Some MUAs put filename only in Content-Type 'name' parameter
		_, ctParams, _ := mime.ParseMediaType(header.Get("Content-Type"))
		filename = ctParams["name"]
	}

	if dispType == "inline" {
		return nil // ignore inline parts
	}
	if dispType == "attachment" || filename != "" {
		m.Attachments = append(m.Attachments, &Attachment{
			Filename:    decodeHeader(filename),
			ContentType: mediaType,
			Header:      header,
			Content:     decoded,
		})
		return nil
	}

	switch {
	case strings.HasPrefix(mediaType, "text/html"):
		m.HTML = decoded
	default:
		m.Text = decoded
	}
	return nil
}

// decodeContent decodes Content-Transfer-Encoding (base64 / quoted-printable).
// 7bit / 8bit / binary / empty are returned as-is.
func decodeContent(body []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		return base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(bytes.NewReader(body)))
	default:
		return body, nil
	}
}

// decodeHeader decodes RFC 2047 encoded-word headers (Subject, filename, etc).
// Returns the original value on decode failure.
func decodeHeader(s string) string {
	dec := new(mime.WordDecoder)
	out, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return out
}

// parseAddressList parses a comma-separated address list (To, Cc).
// Returns the raw value as a single element on parse failure.
func parseAddressList(s string) []string {
	if s == "" {
		return nil
	}
	addrs, err := mail.ParseAddressList(s)
	if err != nil {
		return []string{s}
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.String())
	}
	return out
}
