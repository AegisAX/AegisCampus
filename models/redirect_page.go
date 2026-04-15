package models

import (
	"errors"
	"time"

	log "github.com/gophish/gophish/logger"
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
func GetRedirectPages(uid int64) ([]RedirectPage, error) {
	var pages []RedirectPage
	err := db.Where("user_id = ?", uid).Order("modified_date desc").Find(&pages).Error
	if err != nil {
		log.Error(err)
		return pages, err
	}
	for i := range pages {
		pages[i].attachVideo()
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
