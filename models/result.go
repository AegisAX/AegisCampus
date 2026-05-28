package models

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net"
	"sort"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/oschwald/maxminddb-golang"
)

type mmCity struct {
	GeoPoint mmGeoPoint `maxminddb:"location"`
}

type mmGeoPoint struct {
	Latitude  float64 `maxminddb:"latitude"`
	Longitude float64 `maxminddb:"longitude"`
}

// Result contains the fields for a result object,
// which is a representation of a target in a campaign.
type Result struct {
	Id           int64     `json:"-"`
	CampaignId   int64     `json:"-"`
	UserId       int64     `json:"-"`
	RId          string    `json:"id"`
	Status       string    `json:"status" sql:"not null"`
	IP           string    `json:"ip"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	SendDate     time.Time `json:"send_date"`
	Reported     bool      `json:"reported" sql:"not null"`
	ModifiedDate time.Time `json:"modified_date"`
	// 새로 추가: 신고자가 남긴 메모
	ReportNote string `json:"report_note"`
	// 새로 추가: 첨부파일 열람 여부
	Executed bool `json:"executed" sql:"not null"`
	// 결과 목록 / Top 10 표시용 국가 정보. result.ip 를 GeoIP 룩업해
	// 응답 시점에 채우며 DB 컬럼은 아니다 (gorm:"-").
	Country    string `json:"country" gorm:"-"`
	CountryISO string `json:"country_iso" gorm:"-"`
	BaseRecipient
}

// 첨부파일 열람(실행) 이벤트 + executed=true 로 저장
func (r *Result) HandleAttachmentExecuted(details EventDetails) error {
	event, err := r.createEvent(EventAttachExecuted, details)
	if err != nil {
		return err
	}
	r.Executed = true
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleTrainingCompleted 는 수강 완료 시 이벤트를 남깁니다.
func (r *Result) HandleTrainingCompleted(details EventDetails) error {
	event, err := r.createEvent(EventTrainingCompleted, details)
	if err != nil {
		return err
	}
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

func (r *Result) createEvent(status string, details interface{}) (*Event, error) {
	e := &Event{Email: r.Email, Message: status}
	if details != nil {
		dj, err := json.Marshal(details)
		if err != nil {
			return nil, err
		}
		e.Details = string(dj)
	}
	AddEvent(e, r.CampaignId)
	return e, nil
}

// HandleEmailSent updates a Result to indicate that the email has been
// successfully sent to the remote SMTP server
func (r *Result) HandleEmailSent() error {
	event, err := r.createEvent(EventSent, nil)
	if err != nil {
		return err
	}
	r.SendDate = event.Time
	r.Status = EventSent
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailError updates a Result to indicate that there was an error when
// attempting to send the email to the remote SMTP server.
func (r *Result) HandleEmailError(err error) error {
	event, err := r.createEvent(EventSendingError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = Error
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailBackoff updates a Result to indicate that the email received a
// temporary error and needs to be retried
func (r *Result) HandleEmailBackoff(err error, sendDate time.Time) error {
	event, err := r.createEvent(EventSendingError, EventError{Error: err.Error()})
	if err != nil {
		return err
	}
	r.Status = StatusRetry
	r.SendDate = sendDate
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailOpened updates a Result in the case where the recipient opened the
// email.
func (r *Result) HandleEmailOpened(details EventDetails) error {
	event, err := r.createEvent(EventOpened, details)
	if err != nil {
		return err
	}
	// Don't update the status if the user already clicked the link
	// or submitted data to the campaign
	if r.Status == EventClicked || r.Status == EventDataSubmit {
		return nil
	}
	r.Status = EventOpened
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleClickedLink updates a Result in the case where the recipient clicked
// the link in an email.
func (r *Result) HandleClickedLink(details EventDetails) error {
	event, err := r.createEvent(EventClicked, details)
	if err != nil {
		return err
	}
	// Don't update the status if the user has already submitted data via the
	// landing page form.
	if r.Status == EventDataSubmit {
		return nil
	}
	r.Status = EventClicked
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleFormSubmit updates a Result in the case where the recipient submitted
// credentials to the form on a Landing Page.
func (r *Result) HandleFormSubmit(details EventDetails) error {
	event, err := r.createEvent(EventDataSubmit, details)
	if err != nil {
		return err
	}
	r.Status = EventDataSubmit
	r.ModifiedDate = event.Time
	return db.Save(r).Error
}

// HandleEmailReport updates a Result in the case where they report a simulated
// phishing email using the HTTP handler.
func (r *Result) HandleEmailReport(details EventDetails) error {
	// details를 JSON 문자열로 직렬화(빈 구조체면 ""가 되지 않게 "null" 회피)
	data := ""
	if b, err := json.Marshal(details); err == nil && string(b) != "null" {
		data = string(b)
	}
	e := &Event{
		Email:   r.Email,
		Message: EventReported, // "Email Reported"
		Details: data,
	}
	if err := AddEvent(e, r.CampaignId); err != nil {
		return err
	}
	r.Reported = true
	r.ModifiedDate = e.Time // AddEvent 안에서 Time이 세팅됨
	return db.Save(r).Error
}

// UpdateGeo updates the latitude and longitude of the result in
// the database given an IP address
func (r *Result) UpdateGeo(addr string) error {
	// Open a connection to the maxmind db
	mmdb, err := maxminddb.Open("static/db/geolite2-city.mmdb")
	if err != nil {
		// F2: 과거 log.Fatal 은 os.Exit 로 피싱+어드민 서버를 동시에 종료시켰다.
		// GeoIP 보강은 비필수이므로 에러만 반환하고, 호출부(setupContext)의
		// 기존 graceful 처리(log.Error 후 계속 진행)에 위임한다.
		return err
	}
	defer mmdb.Close()
	ip := net.ParseIP(addr)
	var city mmCity
	// Get the record
	err = mmdb.Lookup(ip, &city)
	if err != nil {
		return err
	}
	// Update the database with the record information
	r.IP = addr
	r.Latitude = city.GeoPoint.Latitude
	r.Longitude = city.GeoPoint.Longitude
	return db.Save(r).Error
}

func generateResultId() (string, error) {
	const alphaNum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	k := make([]byte, 7)
	for i := range k {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphaNum))))
		if err != nil {
			return "", err
		}
		k[i] = alphaNum[idx.Int64()]
	}
	return string(k), nil
}

// GenerateId generates a unique key to represent the result
// in the database
func (r *Result) GenerateId(tx *gorm.DB) error {
	// Keep trying until we generate a unique key (shouldn't take more than one or two iterations)
	for {
		rid, err := generateResultId()
		if err != nil {
			return err
		}
		r.RId = rid
		err = tx.Table("results").Where("r_id=?", r.RId).First(&Result{}).Error
		if err == gorm.ErrRecordNotFound {
			break
		}
	}
	return nil
}

// GetResult returns the Result object from the database
// given the ResultId
func GetResult(rid string) (Result, error) {
	r := Result{}
	err := db.Where("r_id=?", rid).First(&r).Error
	return r, err
}

func GetResultByRID(rid string) (*Result, error) {
	var res Result
	err := db.Where("r_id = ?", rid).First(&res).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &res, err
}

// UnreportResult 는 reported 상태를 false로 되돌리고
// 해당 결과의 "Reported" 이벤트를 타임라인에서 모두 삭제합니다.
func UnreportResult(rid string) error {
	// rid로 Result 조회 (campaign_id, email 확인용)
	r := Result{}
	if err := db.Where("r_id = ?", rid).First(&r).Error; err != nil {
		return err
	}

	// reported = false, report_note = "" 동시 업데이트
	if err := db.Model(&Result{}).
		Where("r_id = ?", rid).
		Updates(map[string]interface{}{
			"reported":    false,
			"report_note": "",
		}).Error; err != nil {
		return err
	}

	// 타임라인에서 해당 이메일의 "Reported" 이벤트 삭제
	return db.Where(
		"campaign_id = ? AND email = ? AND message = ?",
		r.CampaignId, r.Email, EventReported,
	).Delete(&Event{}).Error
}

// CountryStat 은 캠페인별 국가 접속 통계를 표현합니다.
type CountryStat struct {
	Country string `json:"country"`
	ISO     string `json:"iso"`
	Count   int    `json:"count"`
}

// GeoIP DB 의 country 정보를 읽기 위한 구조체 (mmCity 와 별도)
type mmCountryRecord struct {
	Country mmCountry `maxminddb:"country"`
}

type mmCountry struct {
	ISOCode string            `maxminddb:"iso_code"`
	Names   map[string]string `maxminddb:"names"`
}

// resolveCountry 는 IP 문자열을 GeoIP 룩업해 (국가명, ISO) 를 반환합니다.
// 룩업 실패/미등록 IP 면 빈 문자열을 돌려줍니다.
func resolveCountry(mmdb *maxminddb.Reader, ipStr string) (string, string) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", ""
	}
	var rec mmCountryRecord
	if err := mmdb.Lookup(ip, &rec); err != nil {
		return "", ""
	}
	if rec.Country.ISOCode == "" {
		return "", ""
	}
	name := rec.Country.Names["en"]
	if name == "" {
		name = rec.Country.ISOCode
	}
	return name, rec.Country.ISOCode
}

// AttachCountries 는 result.ip 를 GeoIP 룩업해 각 Result 의 Country/CountryISO
// 를 in-place 로 채웁니다. GeoIP DB 가 없으면 (비필수) 조용히 넘어갑니다.
// 같은 IP 는 캐시해 중복 룩업을 피합니다.
func AttachCountries(results []Result) {
	mmdb, err := maxminddb.Open("static/db/geolite2-city.mmdb")
	if err != nil {
		return
	}
	defer mmdb.Close()

	type cc struct{ name, iso string }
	cache := make(map[string]cc)
	for i := range results {
		ipStr := results[i].IP
		if ipStr == "" {
			continue
		}
		if c, ok := cache[ipStr]; ok {
			results[i].Country = c.name
			results[i].CountryISO = c.iso
			continue
		}
		name, iso := resolveCountry(mmdb, ipStr)
		cache[ipStr] = cc{name, iso}
		results[i].Country = name
		results[i].CountryISO = iso
	}
}

// GetCountryStatsByCampaign 은 캠페인 결과의 result.ip 를 GeoIP 룩업해
// 국가별 카운트 Top 10 을 반환합니다. ip 가 있는 result 를 국가 단위로
// 집계하므로, 결과 목록의 국가 컬럼 및 map bubble 과 동일한 기준입니다.
// (ip 가 없거나 GeoIP 미등록 IP 인 result 는 제외 — 합계는 그만큼 작아짐)
func GetCountryStatsByCampaign(uid, campaignId int64) ([]CountryStat, error) {
	var results []Result
	if err := db.Where("campaign_id = ? AND user_id = ? AND ip != ''", campaignId, uid).
		Find(&results).Error; err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []CountryStat{}, nil
	}

	mmdb, err := maxminddb.Open("static/db/geolite2-city.mmdb")
	if err != nil {
		return nil, err
	}
	defer mmdb.Close()

	type countryAgg struct {
		Name  string
		Count int
	}
	agg := make(map[string]*countryAgg) // key: ISO

	for _, r := range results {
		name, iso := resolveCountry(mmdb, r.IP)
		if iso == "" {
			continue
		}
		if a, ok := agg[iso]; ok {
			a.Count++
		} else {
			agg[iso] = &countryAgg{Name: name, Count: 1}
		}
	}

	stats := make([]CountryStat, 0, len(agg))
	for iso, a := range agg {
		stats = append(stats, CountryStat{Country: a.Name, ISO: iso, Count: a.Count})
	}

	// Count 내림차순 정렬 후 Top 10.
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Country < stats[j].Country
	})
	if len(stats) > 10 {
		stats = stats[:10]
	}
	return stats, nil
}
