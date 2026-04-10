// models/video_progress.go
package models

import (
    "time"

    "github.com/jinzhu/gorm"
)

type VideoProgress struct {
    Id             int64     `json:"id" gorm:"primary_key;auto_increment"`
    UserId         int64     `json:"user_id"`
    ResultId       int64     `json:"result_id"`
    VideoId        int64     `json:"video_id"`
    SecondsWatched int64     `json:"seconds_watched"`
    Duration       int64     `json:"duration"`
    Percent        float64   `json:"percent"`
    Completed      bool      `json:"completed"`
    ModifiedDate   time.Time `json:"modified_date"`
}

func (vp *VideoProgress) Save() error {
    vp.ModifiedDate = time.Now().UTC()
    if vp.Id == 0 {
        var existing VideoProgress
        err := db.Where("user_id = ? AND result_id = ? AND video_id = ?",
            vp.UserId, vp.ResultId, vp.VideoId).First(&existing).Error
        if err != nil && err != gorm.ErrRecordNotFound {
            return err
        }
        if existing.Id != 0 {
            vp.Id = existing.Id
        }
    }
    return db.Save(vp).Error  // FirstOrCreate 제거, 원래대로
}

func GetVideoProgress(userId, resultId, videoId int64) (*VideoProgress, error) {
    var vp VideoProgress
    err := db.Where("user_id = ? AND result_id = ? AND video_id = ?", userId, resultId, videoId).First(&vp).Error
    if err == gorm.ErrRecordNotFound {
        return nil, nil
    }
    return &vp, err
}

