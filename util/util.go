package util

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/csv"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"

	"unicode/utf8"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/jordan-wright/email"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	nameRegex       = regexp.MustCompile(`(?i)[\s_-]*name`)
	departmentRegex = regexp.MustCompile(`(?i)[\s_-]*department`)
	emailRegex      = regexp.MustCompile(`(?i)email`)
	positionRegex   = regexp.MustCompile(`(?i)position`)
)

// ParseMail takes in an HTTP Request and returns an Email object
// TODO: This function will likely be changed to take in a []byte
func ParseMail(r *http.Request) (email.Email, error) {
	e := email.Email{}
	m, err := mail.ReadMessage(r.Body)
	if err != nil {
		fmt.Println(err)
	}
	body, err := ioutil.ReadAll(m.Body)
	e.HTML = body
	return e, err
}

// ParseCSV contains the logic to parse the user provided csv file containing Target entries
// 개선 사항:
//   - 인코딩 자동 처리(UTF-8(BOM), CP949/EUC-KR, UTF-16 BOM)
//   - 구분자 자동 추정(, / \t / ; / |)
//   - 헤더 유무 자동 판단 + 헤더/무헤더 모두 처리
//   - 첫 셀 BOM 제거, 널문자 제거, 좌우 공백 정리
func ParseCSV(r *http.Request) ([]models.Target, error) {
	ts := []models.Target{}

	// 1) 멀티파트 업로드 우선 처리
	mr, mrErr := r.MultipartReader()
	if mrErr == nil {
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			// Skip non-file parts
			if part.FileName() == "" {
				continue
			}
			defer part.Close()

			// part 전체를 바이트로 읽고 → UTF-8로 정규화
			raw, err := io.ReadAll(part)
			if err != nil {
				return ts, err
			}
			utf8r, _ := bestEffortUTF8(bytes.NewReader(raw))
			utf8b, err := io.ReadAll(utf8r)
			if err != nil {
				return ts, errors.New("failed to read uploaded data")
			}
			text := string(utf8b)

			// 구분자 추정
			delim := sniffDelimiter(text)

			// 파싱하여 누적
			items, err := parseTargetsFromCSV(text, delim)
			if err != nil {
				return ts, err
			}
			ts = append(ts, items...)
		}
		return ts, nil
	}

	// 2) 멀티파트가 아니면 raw body를 CSV로 가정
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return ts, err
	}
	if len(bodyBytes) == 0 {
		return ts, nil
	}
	utf8r, _ := bestEffortUTF8(bytes.NewReader(bodyBytes))
	utf8b, err := io.ReadAll(utf8r)
	if err != nil {
		return ts, errors.New("failed to read uploaded data")
	}
	text := string(utf8b)
	delim := sniffDelimiter(text)
	return parseTargetsFromCSV(text, delim)
}

// CheckAndCreateSSL is a helper to setup self-signed certificates for the administrative interface.
func CheckAndCreateSSL(cp string, kp string) error {
	// Check whether there is an existing SSL certificate and/or key, and if so, abort execution of this function
	if _, err := os.Stat(cp); !os.IsNotExist(err) {
		return nil
	}
	if _, err := os.Stat(kp); !os.IsNotExist(err) {
		return nil
	}

	log.Infof("Creating new self-signed certificates for administration interface")

	priv, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return fmt.Errorf("error generating tls private key: %v", err)
	}

	notBefore := time.Now()
	// Generate a certificate that lasts for 10 years
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("tls certificate generation: failed to generate a random serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Gophish"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return fmt.Errorf("tls certificate generation: failed to create certificate: %s", err)
	}

	certOut, err := os.Create(cp)
	if err != nil {
		return fmt.Errorf("tls certificate generation: failed to open %s for writing: %s", cp, err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(kp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("tls certificate generation: failed to open %s for writing", kp)
	}

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("tls certificate generation: unable to marshal ECDSA private key: %v", err)
	}

	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b})
	keyOut.Close()

	log.Info("TLS Certificate Generation complete")
	return nil
}

/* =========================
   Helpers for CSV parsing
   ========================= */

func bestEffortUTF8(r io.Reader) (io.Reader, string) {
	br := bufio.NewReader(r)
	// UTF 판별을 위해 조금 넉넉히 미리보기
	peek, _ := br.Peek(4096)

	// UTF-16 BOM
	if len(peek) >= 2 && ((peek[0] == 0xFF && peek[1] == 0xFE) || (peek[0] == 0xFE && peek[1] == 0xFF)) {
		return transform.NewReader(br, unicode.BOMOverride(
			unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder(),
		)), "utf-16"
	}
	// UTF-8 BOM
	if len(peek) >= 3 && peek[0] == 0xEF && peek[1] == 0xBB && peek[2] == 0xBF {
		return br, "utf-8-sig"
	}
	// ★ BOM이 없어도 샘플이 UTF-8로 유효하면 그대로 통과
	if utf8.Valid(peek) {
		return br, "utf-8"
	}
	// 그 외엔 Excel CSV(한글)에서 흔한 CP949/EUC-KR로 가정
	return transform.NewReader(br, korean.EUCKR.NewDecoder()), "cp949/euc-kr"
}

// 첫 비어있지 않은 줄을 기준으로 구분자 추정
func sniffDelimiter(text string) rune {
	cands := []rune{',', '\t', ';', '|'}
	lines := strings.Split(text, "\n")
	first := ""
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			first = ln
			break
		}
	}
	if first == "" {
		return ','
	}
	best := ','
	bestCnt := -1
	for _, d := range cands {
		cnt := strings.Count(first, string(d))
		if cnt > bestCnt {
			bestCnt = cnt
			best = d
		}
	}
	if bestCnt <= 0 {
		return ','
	}
	return best
}

// 헤더처럼 보이는지(간단 휴리스틱)
func looksLikeHeader(row []string) bool {
	joined := strings.ToLower(strings.Join(row, " "))
	// '@'가 없으면 헤더로 간주
	return !(strings.Contains(joined, "@"))
}

// 헤더 별칭(국/영문 혼용)으로 컬럼 인덱스 찾기
func findIndex(header []string, aliases ...string) int {
	for i, h := range header {
		lh := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\uFEFF")))
		for _, a := range aliases {
			if lh == a {
				return i
			}
		}
	}
	return -1
}

// 문자열 안전 접근 및 정리
func pick(rec []string, idx int) string {
	if idx >= 0 && idx < len(rec) {
		return strings.TrimSpace(strings.Trim(rec[idx], "\u0000"))
	}
	return ""
}

// 무헤더일 때 이메일 칼럼 추정(@ 포함)
func guessEmailIndex(rec []string) int {
	best := -1
	for i, v := range rec {
		if strings.Contains(v, "@") {
			best = i
			break
		}
	}
	return best
}

// CSV 텍스트를 파싱해 []models.Target 반환
// (헤더/무헤더 모두 처리)
func parseTargetsFromCSV(text string, delim rune) ([]models.Target, error) {
	out := []models.Target{}

	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true

	// 전체 읽기 (헤더 판단을 위해)
	rows, err := r.ReadAll()
	if err != nil {
		return out, errors.New("failed to parse CSV")
	}
	if len(rows) == 0 {
		return out, nil
	}

	// 1) 헤더 판단
	header := rows[0]
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	}
	dataStart := 1 // 기본: 헤더 있음

	// 헤더 매칭 (원본 정규식 유지)
	fi := -1 // Name
	li := -1 // Department
	ei := -1 // Email
	pi := -1 // Position

	for i, v := range header {
		switch {
		case nameRegex.MatchString(v):
			fi = i
		case departmentRegex.MatchString(v):
			li = i
		case emailRegex.MatchString(v):
			ei = i
		case positionRegex.MatchString(v):
			pi = i
		}
	}

	if fi == -1 && li == -1 && ei == -1 && pi == -1 {
		// 헤더 컬럼을 인식하지 못함 → 무헤더로 간주
		// I-09: 위치 기반 추정(이메일 인접 칼럼)은 실제 CSV 배치와 다를 수 있음
		// 사용자에게 헤더 추가를 권장하는 것이 안전하지만, 하위 호환을 위해 경고만 남김
		log.Warn("CSV에서 헤더를 인식하지 못했습니다. 위치 기반 추정으로 파싱합니다. " +
			"오분류 방지를 위해 'Email', 'Name', 'Department' 헤더 행을 추가하는 것을 권장합니다.")
		dataStart = 0
	}

	// 2) 데이터 행 파싱
	for i := dataStart; i < len(rows); i++ {
		rec := rows[i]

		// 무헤더 모드: 0..n칼럼에 대한 기본 해석
		if dataStart == 0 {
			// 이메일 칼럼 추정
			eIdx := guessEmailIndex(rec)
			if eIdx < 0 || eIdx >= len(rec) {
				continue
			}
			// 이름/부서/직책 기본 매핑: 이메일 주변 칼럼을 관대히 수용
			var fn, ln, ea, ps string

			// 이메일 파싱/정리
			if addr, err := mail.ParseAddress(rec[eIdx]); err == nil {
				ea = addr.Address
			} else {
				ea = strings.TrimSpace(rec[eIdx])
			}
			// 왼쪽/오른쪽 인접 칼럼을 힌트로 사용
			if eIdx-1 >= 0 {
				fn = strings.TrimSpace(rec[eIdx-1])
			}
			if eIdx+1 < len(rec) {
				ln = strings.TrimSpace(rec[eIdx+1])
			}
			if eIdx+2 < len(rec) {
				ps = strings.TrimSpace(rec[eIdx+2])
			}

			if ea == "" {
				continue
			}
			t := models.Target{
				BaseRecipient: models.BaseRecipient{
					Name:       fn,
					Department: ln, // 원본 코드 변수명/매핑 유지: ln → Department
					Email:      ea,
					Position:   ps,
				},
			}
			out = append(out, t)
			continue
		}

		// 헤더 모드: 인덱스 기반
		var fn, ln, ea, ps string

		if fi != -1 && len(rec) > fi {
			fn = strings.TrimSpace(rec[fi])
		}
		if li != -1 && len(rec) > li {
			ln = strings.TrimSpace(rec[li])
		}
		if ei != -1 && len(rec) > ei {
			if csvEmail, err := mail.ParseAddress(rec[ei]); err == nil {
				ea = csvEmail.Address
			} else {
				ea = strings.TrimSpace(rec[ei])
			}
		}
		if pi != -1 && len(rec) > pi {
			ps = strings.TrimSpace(rec[pi])
		}

		if ea == "" {
			continue
		}
		t := models.Target{
			BaseRecipient: models.BaseRecipient{
				Name:       fn,
				Department: ln, // 원본과 동일하게 'li'를 Department로 사용
				Email:      ea,
				Position:   ps,
			},
		}
		out = append(out, t)
	}
	return out, nil
}
