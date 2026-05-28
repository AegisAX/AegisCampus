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
	return db.Save(vp).Error // FirstOrCreate 제거, 원래대로
}

func GetVideoProgress(userId, resultId, videoId int64) (*VideoProgress, error) {
	var vp VideoProgress
	err := db.Where("user_id = ? AND result_id = ? AND video_id = ?", userId, resultId, videoId).First(&vp).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &vp, err
}

// VideoProgressSummary는 캠페인 수강 현황 API의 응답 항목입니다.
type VideoProgressSummary struct {
	RId            string    `json:"rid"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	Department     string    `json:"department"`
	VideoId        int64     `json:"video_id"`
	SecondsWatched int64     `json:"seconds_watched"`
	Duration       int64     `json:"duration"`
	Percent        float64   `json:"percent"`
	Completed      bool      `json:"completed"`
	Trained        bool      `json:"trained"`
	ModifiedDate   time.Time `json:"modified_date"`
}

// GetVideoProgressByCampaign은 특정 캠페인의 수신자별 수강 현황을 반환합니다.
// uid로 캠페인 소유권을 검증합니다.
func GetVideoProgressByCampaign(uid, campaignId int64) ([]VideoProgressSummary, error) {
	var items []VideoProgressSummary
	err := db.Table("video_progresses AS vp").
		Select(`r.r_id, r.email, r.name, r.department,
                vp.video_id, vp.seconds_watched, vp.duration,
                vp.percent, vp.completed,
                EXISTS(
                    SELECT 1 FROM events AS e
                    WHERE e.campaign_id = r.campaign_id
                      AND e.email = r.email
                      AND e.message = 'Trained'
                ) AS trained,
                vp.modified_date`).
		Joins("JOIN results AS r ON r.id = vp.result_id").
		Where("r.campaign_id = ? AND r.user_id = ?", campaignId, uid).
		Order("r.email ASC, vp.video_id ASC").
		Scan(&items).Error
	return items, err
}
