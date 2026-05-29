package models

import (
	"time"

	"github.com/jinzhu/gorm"
)

// UserPreferences 는 사용자별 UI 환경설정 저장소입니다.
//
// (#66) 옵션 C 통합 작업에서 Dashboard 의 "캠페인 선택" 영구 필터를 저장하기
// 위해 도입했습니다. localStorage 가 아닌 DB 저장 — 다른 PC/브라우저에서
// 접속해도 동일한 필터가 유지되도록.
//
// 향후 다른 사용자 환경설정 (테마, 기본 정렬 등) 이 늘면 컬럼만 추가하면
// 됩니다 (테이블 신설 없이).
type UserPreferences struct {
	UserId                  int64     `json:"-" gorm:"primary_key:yes"`
	DashboardCampaignFilter string    `json:"dashboard_campaign_filter"`
	ModifiedDate            time.Time `json:"-"`
}

// TableName 은 GORM 의 복수형 자동 변환 (user_preferences) 과 일치하지만,
// 명시적으로 고정해 향후 모델명이 바뀌어도 테이블명은 안정적이게 한다.
func (UserPreferences) TableName() string {
	return "user_preferences"
}

// GetUserPreferences 는 user_id 의 환경설정을 반환한다.
// 행이 없으면 빈 값 (UserId 만 채워진) 구조체를 반환 — 호출자가 "처음 접속"
// 케이스를 별도 처리할 필요 없게 한다.
func GetUserPreferences(userId int64) (UserPreferences, error) {
	p := UserPreferences{}
	err := db.Where("user_id = ?", userId).First(&p).Error
	if err == gorm.ErrRecordNotFound {
		// 신규 사용자 — 기본값 반환 (저장은 PUT 시점에)
		return UserPreferences{UserId: userId}, nil
	}
	return p, err
}

// SaveUserPreferences 는 user_id 의 환경설정을 upsert 한다.
// First→Save 패턴 (#57 의 race 방지처럼 엄밀할 필요는 없음 — 환경설정은
// 사용자 본인만 동시에 쓸 일이 거의 없고, last-write-wins 면 충분).
func SaveUserPreferences(p *UserPreferences) error {
	if p.UserId == 0 {
		return gorm.ErrRecordNotFound
	}
	p.ModifiedDate = time.Now().UTC()
	existing := UserPreferences{}
	err := db.Where("user_id = ?", p.UserId).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		// INSERT
		return db.Create(p).Error
	}
	if err != nil {
		return err
	}
	// UPDATE — 화이트리스트 패턴 (UserId/ModifiedDate 외 추가 컬럼 늘면 여기 추가)
	return db.Model(&UserPreferences{}).
		Where("user_id = ?", p.UserId).
		Updates(map[string]interface{}{
			"dashboard_campaign_filter": p.DashboardCampaignFilter,
			"modified_date":             p.ModifiedDate,
		}).Error
}

// DeleteUserPreferences 는 user_id 의 환경설정 행을 제거한다.
// (#66) DeleteUser cascade 에서 호출.
func DeleteUserPreferences(userId int64) error {
	return db.Where("user_id = ?", userId).Delete(&UserPreferences{}).Error
}
