package models

import (
    "errors"
    "time"

    "github.com/jinzhu/gorm"
    log "github.com/gophish/gophish/logger"
)

type Video struct {
    Id              int64     `json:"id" gorm:"column:id; primary_key:yes"`
    UserId          int64     `json:"user_id" gorm:"column:user_id"`
    Name            string    `json:"name" gorm:"column:name"`
    Description     string    `json:"description" gorm:"column:description"`
    FileName        string    `json:"file_name" gorm:"column:file_name"`
    FilePath        string    `json:"file_path" gorm:"column:file_path"`
    ThumbnailPath   string    `json:"thumbnail_path" gorm:"column:thumbnail_path"`
    DurationSeconds int       `json:"duration_seconds" gorm:"column:duration_seconds"`
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
    q := db
    if userId != 0 {
        q = q.Where("user_id = ?", userId)
    }
    if err := q.Order("modified_date desc").Find(&videos).Error; err != nil {
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

