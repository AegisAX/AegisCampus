package models

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/AegisAX/Sentinel/logger"
	"github.com/AegisAX/Sentinel/mailer"
	"github.com/AegisAX/Sentinel/util/mimeutil"
)

// MaxSendAttempts set to 8 since we exponentially backoff after each failed send
// attempt. This will give us a maximum send delay of 256 minutes, or about 4.2 hours.
var MaxSendAttempts = 8

// ErrMaxSendAttempts is thrown when the maximum number of sending attempts for a given
// MailLog is exceeded.
var ErrMaxSendAttempts = errors.New("max send attempts exceeded")

// Attachments with these file extensions have inline disposition
var embeddedFileExtensions = []string{".jpg", ".jpeg", ".png", ".gif"}

// MailLog is a struct that holds information about an email that is to be
// sent out.
type MailLog struct {
	Id          int64     `json:"-"`
	UserId      int64     `json:"-"`
	CampaignId  int64     `json:"campaign_id"`
	RId         string    `json:"id"`
	SendDate    time.Time `json:"send_date"`
	SendAttempt int       `json:"send_attempt"`
	Processing  bool      `json:"-"`

	cachedCampaign *Campaign
}

// GenerateMailLog creates a new maillog for the given campaign and
// result. It sets the initial send date to match the campaign's launch date.
func GenerateMailLog(c *Campaign, r *Result, sendDate time.Time) error {
	m := &MailLog{
		UserId:     c.UserId,
		CampaignId: c.Id,
		RId:        r.RId,
		SendDate:   sendDate,
	}
	return db.Save(m).Error
}

// Backoff sets the MailLog SendDate to be the next entry in an exponential
// backoff. ErrMaxRetriesExceeded is thrown if this maillog has been retried
// too many times. Backoff also unlocks the maillog so that it can be processed
// again in the future.
func (m *MailLog) Backoff(reason error) error {
	r, err := GetResult(m.RId)
	if err != nil {
		return err
	}
	if m.SendAttempt == MaxSendAttempts {
		r.HandleEmailError(ErrMaxSendAttempts)
		return ErrMaxSendAttempts
	}
	// Add an error, since we had to backoff because of a
	// temporary error of some sort during the SMTP transaction
	m.SendAttempt++
	backoffDuration := math.Pow(2, float64(m.SendAttempt))
	m.SendDate = m.SendDate.Add(time.Minute * time.Duration(backoffDuration))
	err = db.Save(m).Error
	if err != nil {
		return err
	}
	err = r.HandleEmailBackoff(reason, m.SendDate)
	if err != nil {
		return err
	}
	err = m.Unlock()
	return err
}

// Unlock removes the processing flag so the maillog can be processed again
func (m *MailLog) Unlock() error {
	m.Processing = false
	return db.Save(&m).Error
}

// Lock sets the processing flag so that other processes cannot modify the maillog
func (m *MailLog) Lock() error {
	m.Processing = true
	return db.Save(&m).Error
}

// Error sets the error status on the models.Result that the
// maillog refers to. Since MailLog errors are permanent,
// this action also deletes the maillog.
func (m *MailLog) Error(e error) error {
	r, err := GetResult(m.RId)
	if err != nil {
		log.Warn(err)
		return err
	}
	err = r.HandleEmailError(e)
	if err != nil {
		log.Warn(err)
		return err
	}
	err = db.Delete(m).Error
	return err
}

// Success deletes the maillog from the database and updates the underlying
// campaign result.
func (m *MailLog) Success() error {
	r, err := GetResult(m.RId)
	if err != nil {
		return err
	}
	err = r.HandleEmailSent()
	if err != nil {
		return err
	}
	err = db.Delete(m).Error
	return err
}

// GetDialer returns a dialer based on the maillog campaign's SMTP configuration
func (m *MailLog) GetDialer() (mailer.Dialer, error) {
	c := m.cachedCampaign
	if c == nil {
		campaign, err := GetCampaignMailContext(m.CampaignId, m.UserId)
		if err != nil {
			return nil, err
		}
		c = &campaign
	}
	return c.SMTP.GetDialer()
}

// CacheCampaign allows bulk-mail workers to cache the otherwise expensive
// campaign lookup operation by providing a pointer to the campaign here.
func (m *MailLog) CacheCampaign(campaign *Campaign) error {
	if campaign.Id != m.CampaignId {
		return fmt.Errorf("incorrect campaign provided for caching. expected %d got %d", m.CampaignId, campaign.Id)
	}
	m.cachedCampaign = campaign
	return nil
}

func (m *MailLog) GetSmtpFrom() (string, error) {
	c, err := GetCampaign(m.CampaignId, m.UserId)
	if err != nil {
		return "", err
	}

	f, err := mail.ParseAddress(c.SMTP.FromAddress)
	return f.Address, err
}

// Generate fills in the details of a mailer.Message instance with
// the correct headers and body from the campaign and recipient listed in
// the maillog. We accept the mailer.Message as an argument so that the caller
// can choose to re-use the message across recipients.
func (m *MailLog) Generate(msg *mailer.Message) error {
	r, err := GetResult(m.RId)
	if err != nil {
		return err
	}
	c := m.cachedCampaign
	if c == nil {
		campaign, err := GetCampaignMailContext(m.CampaignId, m.UserId)
		if err != nil {
			return err
		}
		c = &campaign
	}

	f, err := mail.ParseAddress(c.Template.EnvelopeSender)
	if err != nil {
		f, err = mail.ParseAddress(c.SMTP.FromAddress)
		if err != nil {
			return err
		}
	}
	msg.SetAddressHeader("From", f.Address, f.Name)

	ptx, err := NewPhishingTemplateContext(c, r.BaseRecipient, r.RId)
	if err != nil {
		return err
	}

	// Add the transparency headers
	if conf.ContactAddress != "" {
		msg.SetHeader("X-Sentinel-Contact", conf.ContactAddress)
	}

	// Add Message-Id header as described in RFC 2822.
	// messageID, err := m.generateMessageID()
	// if err != nil {
	// return err
	// }
	// msg.SetHeader("Message-Id", messageID)

	// Date 헤더
	msg.SetHeader("Date", time.Now().Format(time.RFC1123Z))
	// Gmail 550-5.7.1 회피: FQDN 기반 Message-ID 생성
	msgidDomain := pickMsgIDDomain(c.Template.EnvelopeSender, c.SMTP.FromAddress)
	messageID, err := generateMessageIDForDomain(msgidDomain)
	if err != nil {
		return err
	}
	// 표준 케이스로 설정 (케이스는 무관하지만 일관성 유지)
	msg.SetHeader("Message-ID", messageID)

	// Parse the customHeader templates
	for _, header := range c.SMTP.Headers {
		key, err := ExecuteTemplate(header.Key, ptx)
		if err != nil {
			log.Warn(err)
		}

		value, err := ExecuteTemplate(header.Value, ptx)
		if err != nil {
			log.Warn(err)
		}

		// Add our header immediately
		// msg.SetHeader(key, value)
		// Message-ID/Date는 덮어쓰기 금지
		kl := strings.ToLower(strings.TrimSpace(key))
		if kl == "message-id" || kl == "messageid" || kl == "date" {
			continue
		}
		msg.SetHeader(key, value)
	}

	// Parse remaining templates
	subject, err := ExecuteTemplate(c.Template.Subject, ptx)

	if err != nil {
		log.Warn(err)
	}
	// don't set Subject header if the subject is empty
	if subject != "" {
		// 비-ASCII 제목을 RFC 2047로 강제 인코딩
		msg.SetHeader("Subject", mimeutil.EncodeHeaderRFC2047(subject))
	}

	// To: 표시명에 비-ASCII가 있을 수 있으니 안전 포맷 적용
	if addr, err := mail.ParseAddress(r.FormatAddress()); err == nil {
		msg.SetAddressHeader("To", addr.Address, addr.Name)
	} else {
		msg.SetHeader("To", r.FormatAddress())
	}
	if c.Template.Text != "" {
		text, err := ExecuteTemplate(c.Template.Text, ptx)
		if err != nil {
			log.Warn(err)
		}
		msg.SetBody("text/plain", text)
	}
	if c.Template.HTML != "" {
		html, err := ExecuteTemplate(c.Template.HTML, ptx)
		if err != nil {
			log.Warn(err)
		}
		if c.Template.Text == "" {
			msg.SetBody("text/html", html)
		} else {
			msg.AddAlternative("text/html", html)
		}
	}
	// Attach the files
	for i := range c.Template.Attachments {
		addAttachment(msg, &c.Template.Attachments[i], ptx) // 원본 포인터 전달
	}

	return nil
}

// GetQueuedMailLogs returns the mail logs that are queued up for the given minute.
func GetQueuedMailLogs(t time.Time) ([]*MailLog, error) {
	ms := []*MailLog{}
	err := db.Where("send_date <= ? AND processing = ?", t, false).
		Find(&ms).Error
	if err != nil {
		log.Warn(err)
	}
	return ms, err
}

// GetMailLogsByCampaign returns all of the mail logs for a given campaign.
func GetMailLogsByCampaign(cid int64) ([]*MailLog, error) {
	ms := []*MailLog{}
	err := db.Where("campaign_id = ?", cid).Find(&ms).Error
	return ms, err
}

// LockMailLogs locks or unlocks a slice of maillogs for processing.
func LockMailLogs(ms []*MailLog, lock bool) error {
	tx := db.Begin()
	for i := range ms {
		ms[i].Processing = lock
		err := tx.Save(ms[i]).Error
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()
	return nil
}

// UnlockAllMailLogs removes the processing lock for all maillogs
// in the database. This is intended to be called when Sentinel is started
// so that any previously locked maillogs can resume processing.
func UnlockAllMailLogs() error {
	return db.Model(&MailLog{}).Update("processing", false).Error
}

// Message-ID 헤더에 사용할 도메인 결정
// 우선순위:
//  1. 환경변수 SENTINEL_MSGID_DOMAIN
//  2. Template EnvelopeSender의 @ 뒤 도메인
//  3. SMTP FromAddress의 @ 뒤 도메인
//  4. os.Hostname() — 점(.)이 있는 FQDN인 경우만
//  5. "mail.invalid" — 위 모두 실패 시
//
// 환경변수 예시: export SENTINEL_MSGID_DOMAIN=mail.example.com
func pickMsgIDDomain(envelopeSender, smtpFromAddress string) string {
	// 1) 환경변수 우선
	if v := os.Getenv("SENTINEL_MSGID_DOMAIN"); v != "" && strings.Contains(v, ".") {
		return strings.ToLower(v)
	}
	// 2) EnvelopeSender 도메인
	if addr, err := mail.ParseAddress(envelopeSender); err == nil {
		parts := strings.Split(addr.Address, "@")
		if len(parts) == 2 && strings.Contains(parts[1], ".") {
			return strings.ToLower(parts[1])
		}
	}
	// 3) SMTP.FromAddress 도메인
	if addr, err := mail.ParseAddress(smtpFromAddress); err == nil {
		parts := strings.Split(addr.Address, "@")
		if len(parts) == 2 && strings.Contains(parts[1], ".") {
			return strings.ToLower(parts[1])
		}
	}
	// 4) os.Hostname() — 점(.)이 있는 진짜 FQDN인 경우만 사용
	if h, err := os.Hostname(); err == nil && strings.Contains(h, ".") {
		return strings.ToLower(h)
	}
	// 5) 설정이 필요함을 명시하는 기본값
	// "mail.invalid" 는 RFC 2606에 따라 예약된 도메인으로, 실수로 외부에 메일이
	// 전달되는 것을 방지하면서 설정 누락을 명확히 표시합니다.
	return "mail.invalid"
}

// generateMessageIDForDomain: FQDN을 사용해 RFC5322 규격 Message-ID 생성.
// 패키지 함수로 두어 MailLog (캠페인 경로) 와 EmailRequest (테스트 메일 경로)
// 양쪽에서 공유 호출한다.
func generateMessageIDForDomain(domain string) (string, error) {
	t := time.Now().UnixNano()
	pid := os.Getpid()
	rint, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<%d.%d.%d@%s>", t, pid, rint, domain), nil
}

var maxBigInt = big.NewInt(math.MaxInt64)

// generateMessageID generates and returns a string suitable for an RFC 2822
// compliant Message-ID, e.g.:
// <1444789264909237300.3464.1819418242800517193@DESKTOP01>
//
// The following parameters are used to generate a Message-ID:
// - The nanoseconds since Epoch
// - The calling PID
// - A cryptographically random int64
// - The sending hostname
func (m *MailLog) generateMessageID() (string, error) {
	t := time.Now().UnixNano()
	pid := os.Getpid()
	rint, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return "", err
	}
	h, err := os.Hostname()
	// If we can't get the hostname, we'll use localhost
	if err != nil {
		h = "localhost.localdomain"
	}
	msgid := fmt.Sprintf("<%d.%d.%d@%s>", t, pid, rint, h)
	return msgid, nil
}

// Check if an attachment should have inline disposition based on
// its file extension.
func shouldEmbedAttachment(name string) bool {
	ext := filepath.Ext(name)
	for _, v := range embeddedFileExtensions {
		if strings.EqualFold(ext, v) {
			return true
		}
	}
	return false
}

// Add an attachment to a mailer message.
// 첨부 파트의 헤더에 RFC2231 filename* / name*을 추가하여
// 헤더에 비-ASCII가 남지 않도록 처리한다.
func addAttachment(msg *mailer.Message, a *Attachment, ptx PhishingTemplateContext) {
	copyFunc := mailer.SetCopyFunc(func(w io.Writer) error {
		reader, err := a.ApplyTemplate(ptx) // a는 원본 포인터 → vanillaFile 반영
		if err != nil {
			return err
		}
		_, err = io.Copy(w, reader)
		return err
	})

	inline := shouldEmbedAttachment(a.Name)
	// mailer 내부 기본 처리에서 비-ASCII를 건드리지 않도록, 라이브러리 인자도 ASCII로 전달
	asciiName := mimeutil.PercentASCIIName(a.Name)

	// Content-Type 추정 (확장자 기반)
	ctype := ""
	if ext := strings.ToLower(filepath.Ext(a.Name)); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			ctype = t
		}
	}
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	// RFC2231/5987 + ASCII 폴백 헤더 구성
	hdr := mimeutil.BuildAttachmentHeaders(a.Name, ctype, inline)

	if inline {
		// inline 이미지는 Content-ID가 필요할 수 있는데,
		// 여기서는 Content-ID를 지정하지 않으면 mailer가 자동으로 추가한다.
		msg.Embed(
			asciiName,
			mailer.SetHeader(hdr),
			copyFunc,
		)
	} else {
		// 일부 클라이언트 호환을 위해 폴백 파일명도 지정
		msg.Attach(
			asciiName,
			mailer.SetHeader(hdr),
			copyFunc,
		)
	}
}
