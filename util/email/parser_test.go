package emailparser

import (
	"bytes"
	"strings"
	"testing"
)

// fixture 는 raw RFC 5322 메시지를 NewMessageFromReader 로 파싱한 후
// 기대 결과와 비교하는 회귀 테스트 케이스다.
type fixture struct {
	name    string
	raw     string
	checkFn func(t *testing.T, m *Message)
}

var fixtures = []fixture{
	{
		name: "PlainText",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Hello\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" +
			"Hello, world.\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if m.Subject != "Hello" {
				t.Errorf("Subject: %q", m.Subject)
			}
			if string(m.Text) != "Hello, world.\r\n" {
				t.Errorf("Text: %q", m.Text)
			}
		},
	},
	{
		name: "HTMLOnly",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: HTML\r\n" +
			"Content-Type: text/html; charset=utf-8\r\n" +
			"\r\n" +
			"<p>Hello</p>\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if !bytes.Contains(m.HTML, []byte("<p>Hello</p>")) {
				t.Errorf("HTML missing: %q", m.HTML)
			}
		},
	},
	{
		name: "MultipartAlternative",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Alt\r\n" +
			"Content-Type: multipart/alternative; boundary=BOUND\r\n" +
			"\r\n" +
			"--BOUND\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" +
			"plain body\r\n" +
			"--BOUND\r\n" +
			"Content-Type: text/html; charset=utf-8\r\n" +
			"\r\n" +
			"<p>html body</p>\r\n" +
			"--BOUND--\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if !bytes.Contains(m.Text, []byte("plain body")) {
				t.Errorf("Text missing 'plain body': %q", m.Text)
			}
			if !bytes.Contains(m.HTML, []byte("html body")) {
				t.Errorf("HTML missing 'html body': %q", m.HTML)
			}
		},
	},
	{
		name: "MultipartMixedWithAttachment",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Mixed\r\n" +
			"Content-Type: multipart/mixed; boundary=BOUND\r\n" +
			"\r\n" +
			"--BOUND\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" +
			"body text\r\n" +
			"--BOUND\r\n" +
			"Content-Type: application/octet-stream; name=\"file.bin\"\r\n" +
			"Content-Disposition: attachment; filename=\"file.bin\"\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"\r\n" +
			"SGVsbG8=\r\n" +
			"--BOUND--\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if len(m.Attachments) != 1 {
				t.Fatalf("expected 1 attachment, got %d", len(m.Attachments))
			}
			a := m.Attachments[0]
			if a.Filename != "file.bin" {
				t.Errorf("Filename: %q", a.Filename)
			}
			if string(a.Content) != "Hello" {
				t.Errorf("attachment Content: %q (want 'Hello')", a.Content)
			}
		},
	},
	{
		name: "InlinePartIsIgnored",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: Inline\r\n" +
			"Content-Type: multipart/mixed; boundary=BOUND\r\n" +
			"\r\n" +
			"--BOUND\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"body\r\n" +
			"--BOUND\r\n" +
			"Content-Type: image/png\r\n" +
			"Content-Disposition: inline; filename=\"inline.png\"\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"\r\n" +
			"SGVsbG8=\r\n" +
			"--BOUND--\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if len(m.Attachments) != 0 {
				t.Errorf("inline part should be skipped, got %d attachments", len(m.Attachments))
			}
		},
	},
	{
		name: "KoreanSubjectRFC2047",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: =?utf-8?B?7JWI64WVIO2VmOyEuOyalA==?=\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" +
			"hi\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if m.Subject != "안녕 하세요" {
				t.Errorf("Subject decode: %q", m.Subject)
			}
		},
	},
	{
		name: "KoreanAttachmentFilenameRFC2047",
		raw: "From: alice@example.com\r\n" +
			"To: bob@example.com\r\n" +
			"Subject: file\r\n" +
			"Content-Type: multipart/mixed; boundary=BOUND\r\n" +
			"\r\n" +
			"--BOUND\r\n" +
			"Content-Type: text/plain; charset=utf-8\r\n" +
			"\r\n" +
			"body\r\n" +
			"--BOUND\r\n" +
			"Content-Type: application/pdf; name=\"=?utf-8?B?7Jew7IiYLnBkZg==?=\"\r\n" +
			"Content-Disposition: attachment; filename=\"=?utf-8?B?7Jew7IiYLnBkZg==?=\"\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"\r\n" +
			"SGVsbG8=\r\n" +
			"--BOUND--\r\n",
		checkFn: func(t *testing.T, m *Message) {
			if len(m.Attachments) != 1 {
				t.Fatalf("expected 1 attachment, got %d", len(m.Attachments))
			}
			if m.Attachments[0].Filename != "연수.pdf" {
				t.Errorf("Filename decode: %q", m.Attachments[0].Filename)
			}
		},
	},
}

func TestParserFixtures(t *testing.T) {
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			m, err := NewMessageFromReader(strings.NewReader(fx.raw))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}
			fx.checkFn(t, m)
		})
	}
}
