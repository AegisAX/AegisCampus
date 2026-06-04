package models

import (
	"errors"
	"regexp"
	"strconv"
	"time"

	log "github.com/AegisAX/AegisCampus/logger"
	"github.com/jinzhu/gorm"
)

// RedirectPage 는 피싱 Landing Page 이후에 표시되는 교육/안내 페이지입니다.
// VideoId 를 지정하면 해당 동영상이 페이지 내에 임베드되어 수강 추적까지 연동됩니다.
type RedirectPage struct {
	Id           int64     `json:"id"           gorm:"column:id; primary_key:yes"`
	UserId       int64     `json:"-"            gorm:"column:user_id"`
	Name         string    `json:"name"`
	HTML         string    `json:"html"         gorm:"column:html"`
	VideoId      *int64    `json:"video_id"     gorm:"column:video_id"`
	Video        *Video    `json:"video,omitempty" gorm:"-"` // JOIN 없이 별도 조회
	RedirectURL  string    `json:"redirect_url" gorm:"column:redirect_url"`
	ModifiedDate time.Time `json:"modified_date"`
}

// 오류 정의
var (
	ErrRedirectPageNameNotSpecified = errors.New("Redirect Page name not specified")
	ErrRedirectPageNotFound         = errors.New("redirect page not found")
	ErrRedirectPageNameInUse        = errors.New("Redirect Page name already in use")
)

// Validate 는 저장 전 필드 유효성을 검사합니다.
func (rp *RedirectPage) Validate() error {
	if rp.Name == "" {
		return ErrRedirectPageNameNotSpecified
	}
	// video_id = 0 을 "미설정"으로 정규화
	if rp.VideoId != nil && *rp.VideoId == 0 {
		rp.VideoId = nil
	}
	// (#39) page.go 와 비대칭이던 템플릿 검증 누락 보강
	if err := ValidateTemplate(rp.HTML); err != nil {
		return err
	}
	if err := ValidateTemplate(rp.RedirectURL); err != nil {
		return err
	}
	return nil
}

// attachVideo 는 VideoId 가 설정된 경우 Video 정보를 채웁니다.
func (rp *RedirectPage) attachVideo() {
	if rp.VideoId == nil || *rp.VideoId <= 0 {
		return
	}
	v, err := GetVideo(*rp.VideoId)
	if err == nil {
		rp.Video = v
	}
}

// GetRedirectPages 는 해당 사용자의 모든 Redirect Page를 반환합니다.
// video_id 수집 후 단일 IN 쿼리
func GetRedirectPages(uid int64) ([]RedirectPage, error) {
	var pages []RedirectPage
	err := db.Where("user_id = ?", uid).Order("modified_date desc").Find(&pages).Error
	if err != nil {
		log.Error(err)
		return pages, err
	}

	// video_id 수집
	var videoIDs []int64
	for _, rp := range pages {
		if rp.VideoId != nil && *rp.VideoId > 0 {
			videoIDs = append(videoIDs, *rp.VideoId)
		}
	}

	// 단일 IN 쿼리로 필요한 Video만 조회
	if len(videoIDs) > 0 {
		var videos []Video
		if err := db.Where("id IN (?)", videoIDs).Find(&videos).Error; err != nil {
			// IN 쿼리 실패 시에도 페이지 목록은 반환하되(관리 UI 가용성 우선),
			// Video 첨부 누락을 silent 로 넘기지 않고 로그를 남긴다.
			log.Error(err)
		} else {
			// id → Video 맵 구성
			vmap := make(map[int64]*Video, len(videos))
			for i := range videos {
				vmap[videos[i].Id] = &videos[i]
			}
			// 각 페이지에 매핑
			for i := range pages {
				if pages[i].VideoId != nil {
					pages[i].Video = vmap[*pages[i].VideoId]
				}
			}
		}
	}

	return pages, nil
}

// GetRedirectPage 는 id + user_id 조건으로 단건 조회합니다.
func GetRedirectPage(id int64, uid int64) (RedirectPage, error) {
	rp := RedirectPage{}
	err := db.Where("user_id = ? AND id = ?", uid, id).First(&rp).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return rp, ErrRedirectPageNotFound
		}
		log.Error(err)
		return rp, err
	}
	rp.attachVideo()
	return rp, nil
}

// GetRedirectPageByName 은 이름+사용자 ID 로 단건 조회합니다 (중복 검사용).
func GetRedirectPageByName(name string, uid int64) (RedirectPage, error) {
	rp := RedirectPage{}
	err := db.Where("user_id = ? AND name = ?", uid, name).First(&rp).Error
	return rp, err
}

// PostRedirectPage 는 새 Redirect Page를 저장합니다.
func PostRedirectPage(rp *RedirectPage) error {
	if err := rp.Validate(); err != nil {
		return err
	}
	rp.ModifiedDate = time.Now().UTC()
	if err := db.Save(rp).Error; err != nil {
		log.Error(err)
		return err
	}
	rp.attachVideo()
	return nil
}

// PutRedirectPage 는 기존 Redirect Page를 덮어씁니다.
func PutRedirectPage(rp *RedirectPage) error {
	if err := rp.Validate(); err != nil {
		return err
	}
	rp.ModifiedDate = time.Now().UTC()
	if err := db.Where("id = ?", rp.Id).Save(rp).Error; err != nil {
		log.Error(err)
		return err
	}
	rp.attachVideo()
	return nil
}

// ErrRedirectPageInUse 는 LP 가 RedirectURL 로 참조 중인 RP 삭제 시도 시 반환됩니다.
var ErrRedirectPageInUse = errors.New("Redirect page is in use by one or more landing pages and cannot be deleted")

// IsRedirectPageInUse 는 어떤 랜딩 페이지든 RedirectURL 로 이 RP 를 가리키면 true 를
// 반환합니다(전체 user). LIKE 로 /rp/ 포함 후보를 좁힌 뒤 ExtractRedirectPageID 로
// 정확한 ID 일치를 확정합니다(/rp/3 이 /rp/30 을 오탐하지 않도록).
func IsRedirectPageInUse(id int64) (bool, error) {
	var redirectURLs []string
	err := db.Table("pages").Where("redirect_url LIKE ?", "%/rp/%").Pluck("redirect_url", &redirectURLs).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		log.Error(err)
		return false, err
	}
	for _, u := range redirectURLs {
		if ExtractRedirectPageID(u) == id {
			return true, nil
		}
	}
	return false, nil
}

// DeleteRedirectPage 는 Redirect Page를 삭제합니다.
func DeleteRedirectPage(id int64, uid int64) error {
	inUse, err := IsRedirectPageInUse(id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrRedirectPageInUse
	}
	err = db.Where("user_id = ?", uid).Delete(RedirectPage{Id: id}).Error
	if err != nil {
		log.Error(err)
	}
	return err
}

// GetRedirectPageByID returns a RedirectPage by ID only (no user filter).
// Used by the phishing server to serve the page publicly.
func GetRedirectPageByID(id int64) (RedirectPage, error) {
	rp := RedirectPage{}
	err := db.Where("id = ?", id).First(&rp).Error
	if err != nil {
		return rp, err
	}
	rp.attachVideo()
	return rp, nil
}

// #41: rid↔campaign↔asset 매핑 무결성 검증용 헬퍼.
// LandingPage.RedirectUrl 의 `/rp/{id}` 패턴에서 RP ID 를 추출한다.
// 패턴이 일치하지 않으면 0 을 반환 — 호출자는 "이 캠페인엔 자체 RP 없음"
// 으로 해석한다 (외부 redirect URL 케이스 등).
var redirectPageURLPattern = regexp.MustCompile(`/rp/(\d+)`)

// ExtractRedirectPageID returns the RP ID embedded in a `/rp/{id}` URL,
// or 0 if the URL does not match that pattern.
func ExtractRedirectPageID(redirectURL string) int64 {
	m := redirectPageURLPattern.FindStringSubmatch(redirectURL)
	if len(m) != 2 {
		return 0
	}
	rpID, _ := strconv.ParseInt(m[1], 10, 64)
	return rpID
}
