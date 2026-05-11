package models

import (
	"errors"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/jinzhu/gorm"
)

type Video struct {
	Id              int64     `json:"id" gorm:"column:id; primary_key:yes"`
	UserId          int64     `json:"user_id" gorm:"column:user_id"`
	Name            string    `json:"name" gorm:"column:name"`
	Description     string    `json:"description" gorm:"column:description"`
	FileName        string    `json:"file_name" gorm:"column:file_name"`
	FilePath        string    `json:"file_path" gorm:"column:file_path"`
	ThumbnailPath   string    `json:"thumbnail_path" gorm:"column:thumbnail_path"`
	DurationSeconds int64     `json:"duration_seconds" gorm:"column:duration_seconds"`
	IsPublic        bool      `json:"is_public" gorm:"column:is_public"`
	CreatedDate     time.Time `json:"created_date" gorm:"column:created_date"`
	ModifiedDate    time.Time `json:"modified_date" gorm:"column:modified_date"`
}

var ErrVideoNotFound = errors.New("video not found")

// CreateVideo saves a new video record
func CreateVideo(v *Video) error {
	v.CreatedDate = time.Now().UTC()
	v.ModifiedDate = v.CreatedDate
	if err := db.Create(v).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// UpdateVideo updates an existing video
func UpdateVideo(v *Video) error {
	v.ModifiedDate = time.Now().UTC()
	if err := db.Save(v).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// GetVideo returns a video by ID
func GetVideo(id int64) (*Video, error) {
	v := &Video{}
	if err := db.First(v, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrVideoNotFound
		}
		log.Error(err)
		return nil, err
	}
	return v, nil
}

// GetVideosForUser returns videos for a specific user (or all if user_id==0)
func GetVideosForUser(userId int64) ([]Video, error) {
	var videos []Video
	var err error
	if userId != 0 {
		// 자신의 영상(공개/비공개 무관) + 다른 사용자의 공개 영상
		err = db.Where("user_id = ? OR is_public = ?", userId, true).
			Order("modified_date desc").Find(&videos).Error
	} else {
		err = db.Where("is_public = ?", true).
			Order("modified_date desc").Find(&videos).Error
	}
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return videos, nil
}

// DeleteVideo deletes a video record
func DeleteVideo(id int64) error {
	v := &Video{Id: id}
	if err := db.Delete(v).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}

// 같은 실제 파일(= 동일 해시 파일명)을 참조하는 레코드 수를 센다.
func CountVideosByFileName(fileName string) (int64, error) {
	var cnt int64
	if err := db.Model(&Video{}).Where("file_name = ?", fileName).Count(&cnt).Error; err != nil {
		return 0, err
	}
	return cnt, nil
}

// IsVideoUsedByOthers returns true if any other user's RedirectPage references this video.
// ownerUserId is excluded from the check (owner can always modify their own references).
func IsVideoUsedByOthers(videoId int64, ownerUserId int64) (bool, error) {
	var count int64
	err := db.Model(&RedirectPage{}).
		Where("video_id = ? AND user_id != ?", videoId, ownerUserId).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsVideoInUse returns true if any RedirectPage references this video.
// 소유자 포함 전체 참조를 확인하므로 삭제 전 사용 여부 체크에 사용합니다.
func IsVideoInUse(videoId int64) (bool, error) {
	var count int64

	// Redirect Page 참조 확인
	err := db.Model(&RedirectPage{}).
		Where("video_id = ?", videoId).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	// Landing Page 참조 확인 (추가)
	err = db.Model(&Page{}).
		Where("video_id = ?", videoId).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsVideoLinkedToUser returns true if any LandingPage OR RedirectPage owned by
// userId references this video. Used by the public Media handler (Phase 2 #7)
// to authorize rid-based access where the video may be embedded in either the
// campaign's LandingPage (pages.video_id) or its RedirectPage (redirect_pages.video_id).
func IsVideoLinkedToUser(videoId int64, userId int64) (bool, error) {
	var count int64

	// 1) RedirectPage 검사
	err := db.Model(&RedirectPage{}).
		Where("video_id = ? AND user_id = ?", videoId, userId).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	// 2) LandingPage 검사
	err = db.Model(&Page{}).
		Where("video_id = ? AND user_id = ?", videoId, userId).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
