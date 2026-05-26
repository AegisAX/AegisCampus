package models

import (
	"errors"
	"regexp"
	"strconv"
	"time"

	log "github.com/AegisAX/Sentinel/logger"
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
		if err := db.Where("id IN (?)", videoIDs).Find(&videos).Error; err == nil {
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

// DeleteRedirectPage 는 Redirect Page를 삭제합니다.
func DeleteRedirectPage(id int64, uid int64) error {
	err := db.Where("user_id = ?", uid).Delete(RedirectPage{Id: id}).Error
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
