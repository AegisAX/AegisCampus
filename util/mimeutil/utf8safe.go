package mimeutil

import (
	"fmt"
	"mime"
	"net/mail"
	"net/url"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// 비-ASCII 문자열을 RFC 2047로 강제 인코딩 (주로 Subject 등)
func EncodeHeaderRFC2047(s string) string {
	if isASCII(s) {
		return s
	}
	return mime.QEncoding.Encode("utf-8", s)
}

// EncodeHeaderRFC2047B: Base64(B) 인코딩으로 encoded-word 생성 (일부 환경에서 더 호환적)
func EncodeHeaderRFC2047B(s string) string {
	if isASCII(s) {
		return s
	}
	return mime.BEncoding.Encode("utf-8", s)
}

// 표시명이 비-ASCII여도 RFC 2047로 안전하게 문자열화
func FormatAddressSafe(display, addr string) string {
	return (&mail.Address{Name: display, Address: addr}).String()
}

// 파일명 ASCII 폴백 생성 (확장자는 가능하면 유지)
func ASCIIFileNameFallback(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	base := strings.TrimSuffix(name, ext)
	var b strings.Builder
	for _, r := range base {
		if r < 128 && (isAlphaNum(byte(r)) || strings.ContainsRune(" ._-", r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		b.WriteString("attachment")
	}
	// 만약 전부 밑줄/공백만 남았다면 사람이 읽을 수 있는 기본값으로 교체
	s := b.String()
	trim := strings.Trim(s, " _.-")
	if trim == "" {
		s = "attachment"
	}
	f := strings.TrimSpace(strings.ReplaceAll(s, "  ", " "))
	if len(f) > 60 {
		f = f[:60]
	}
	if ext != "" && len(ext) <= 16 && strings.HasPrefix(ext, ".") && isASCII(ext) {
		return f + ext
	}
	return f
}

// PercentASCIIName: 원본 UTF-8 파일명을 URL percent-encoding으로 ASCII로 변환
func PercentASCIIName(s string) string {
	if s == "" {
		return "attachment"
	}
	if !utf8.ValidString(s) {
		s = ASCIIFileNameFallback(s)
	}
	return url.PathEscape(s) // 공백→%20, 한글 등 비-ASCII → %XX
}

// RFC2231/5987 형식의 filename* / name* 파라미터와 ASCII 폴백을 만드는 헤더
// Content-Type: <ctype>; name="<fallback>"; name*=UTF-8''<pct-utf8>
// Content-Disposition: <disp>; filename="<fallback>"; filename*=UTF-8''<pct-utf8>
func BuildAttachmentHeaders(utf8Filename, contentType string, inline bool) map[string][]string {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// filename=/name= 에는 RFC 2047 encoded-word(=?UTF-8?B?...?=) 사용
	display := EncodeHeaderRFC2047B(utf8Filename)
	star := "UTF-8''" + rfc5987(utf8Filename)
	disp := "attachment"
	if inline {
		disp = "inline"
	}
	// text/* 는 charset을 명시 (일부 MUA 호환성 개선)
	if strings.HasPrefix(strings.ToLower(contentType), "text/") && !strings.Contains(strings.ToLower(contentType), "charset=") {
		contentType += `; charset=utf-8`
	}
	ct := fmt.Sprintf(`%s; name="%s"; name*=%s`, contentType, display, star)
	cd := fmt.Sprintf(`%s; filename="%s"; filename*=%s`, disp, display, star)
	return map[string][]string{
		"Content-Type":        {ct},
		"Content-Disposition": {cd},
	}
}

// ---------------- 내부 유틸 ----------------
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 0x7F {
			return false
		}
	}
	return true
}
func isAlphaNum(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}
// RFC5987 percent-encoding (RFC2231용)
func rfc5987(s string) string {
	if s == "" {
		return ""
	}
	if !utf8.ValidString(s) {
		return url.PathEscape(ASCIIFileNameFallback(s))
	}
	return url.PathEscape(s) // 공백→%20, 비-ASCII→%XX
}

