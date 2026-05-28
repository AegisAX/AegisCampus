// models/video_progress.go
package models

import (
	"strings"
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

// Save는 (user_id, result_id, video_id) 자연키로 upsert 한다.
// First→Save 사이의 race 로 동시 INSERT 가 발생할 수 있어
// (DB 의 UNIQUE 인덱스 idx_video_progresses_unique_urv 가 차단),
// INSERT 가 UNIQUE 위반으로 실패하면 직전에 다른 요청이 만든 행을
// 다시 찾아 그 ID 로 UPDATE 재시도한다.
func (vp *VideoProgress) Save() error {
	vp.ModifiedDate = time.Now().UTC()

	findExisting := func() (int64, error) {
		var existing VideoProgress
		err := db.Where("user_id = ? AND result_id = ? AND video_id = ?",
			vp.UserId, vp.ResultId, vp.VideoId).First(&existing).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return 0, err
		}
		return existing.Id, nil
	}

	if vp.Id == 0 {
		id, err := findExisting()
		if err != nil {
			return err
		}
		if id != 0 {
			vp.Id = id
		}
	}

	err := db.Save(vp).Error
	if err != nil && vp.Id == 0 && isUniqueConstraintErr(err) {
		// race: 다른 요청이 같은 자연키 행을 먼저 INSERT 함.
		// 그 행을 찾아 ID 를 채우고 UPDATE 로 재시도.
		id, ferr := findExisting()
		if ferr != nil {
			return ferr
		}
		if id != 0 {
			vp.Id = id
			return db.Save(vp).Error
		}
	}
	return err
}

// isUniqueConstraintErr 는 SQLite/MySQL 의 UNIQUE 제약 위반 에러를
// 문자열로 식별한다 (드라이버 구조체 타입 의존 회피).
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") || // sqlite: "UNIQUE constraint failed"
		strings.Contains(msg, "duplicate entry") // mysql: "Error 1062: Duplicate entry"
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
