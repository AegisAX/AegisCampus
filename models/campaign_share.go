package models

import (
	"time"

	log "github.com/AegisAX/Sentinel/logger"
	"github.com/jinzhu/gorm"
)

// CampaignShare grants a user read-only access to another user's campaign.
// 캠페인 소유자(campaign.user_id)가 아닌 user_id 가 해당 campaign 의 결과
// 화면을 읽을 수 있도록 부여하는 grant. (campaign_id, user_id) 가 자연키이며
// UNIQUE 인덱스로 중복 INSERT 를 차단한다.
type CampaignShare struct {
	Id          int64     `json:"id"`
	CampaignId  int64     `json:"campaign_id"`
	UserId      int64     `json:"user_id"`
	CreatedDate time.Time `json:"created_date"`
}

// TableName overrides the default GORM pluralization to match the migration.
func (CampaignShare) TableName() string {
	return "campaign_shares"
}

// CanViewCampaign reports whether viewerUid is allowed to read the campaign
// identified by id, and returns the campaign's owner uid for downstream
// queries that still need the owner's user_id (results, etc.).
//
// 허용 조건:
//
//	(1) viewerUid 가 캠페인 소유자
//	(2) campaign_shares 에 (id, viewerUid) 행 존재
//
// 캠페인 자체가 없으면 gorm.ErrRecordNotFound 를 반환한다. 호출자는 이
// 에러를 그대로 404 로 응답해 존재 자체를 비노출한다 (#41 류 정보 노출 방지).
func CanViewCampaign(id int64, viewerUid int64) (bool, int64, error) {
	var ownerUid int64
	row := db.Table("campaigns").Where("id = ?", id).Select("user_id").Row()
	if err := row.Scan(&ownerUid); err != nil {
		// gorm.ErrRecordNotFound 가 아닌 sql.ErrNoRows 가 올 수 있으나,
		// 어느 쪽이든 호출자에선 "찾을 수 없음" 으로 처리하면 충분.
		return false, 0, gorm.ErrRecordNotFound
	}
	if ownerUid == viewerUid {
		return true, ownerUid, nil
	}
	var count int64
	err := db.Model(&CampaignShare{}).
		Where("campaign_id = ? AND user_id = ?", id, viewerUid).
		Count(&count).Error
	if err != nil {
		return false, 0, err
	}
	return count > 0, ownerUid, nil
}

// AddCampaignShare grants viewerUid read-only access to the campaign. The
// UNIQUE(campaign_id, user_id) constraint makes repeated calls a no-op on the
// DB side; we still surface the duplicate error if it happens to occur on
// some drivers (callers should treat it as success).
func AddCampaignShare(campaignId int64, viewerUid int64) error {
	cs := &CampaignShare{
		CampaignId:  campaignId,
		UserId:      viewerUid,
		CreatedDate: time.Now().UTC(),
	}
	return db.Save(cs).Error
}

// DeleteCampaignShare revokes the grant. Missing rows return nil (idempotent).
func DeleteCampaignShare(campaignId int64, viewerUid int64) error {
	return db.Where("campaign_id = ? AND user_id = ?", campaignId, viewerUid).
		Delete(&CampaignShare{}).Error
}

// GetCampaignShares lists current grants for the campaign.
func GetCampaignShares(campaignId int64) ([]CampaignShare, error) {
	shares := []CampaignShare{}
	err := db.Where("campaign_id = ?", campaignId).Find(&shares).Error
	if err != nil {
		log.Error(err)
	}
	return shares, err
}

// GetSharedCampaignIDs returns campaign IDs that have been shared with
// viewerUid (i.e. the viewer is NOT the owner). Used by the campaigns list /
// dashboard to merge shared campaigns into the viewer's view.
func GetSharedCampaignIDs(viewerUid int64) ([]int64, error) {
	rows, err := db.Table("campaign_shares").
		Where("user_id = ?", viewerUid).
		Select("campaign_id").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return ids, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DeleteCampaignSharesByCampaign removes all grants for a campaign. Called
// from DeleteCampaign so that shares don't outlive their campaign.
func DeleteCampaignSharesByCampaign(campaignId int64) error {
	return db.Where("campaign_id = ?", campaignId).Delete(&CampaignShare{}).Error
}

// DeleteCampaignSharesByUser removes all grants where viewerUid was the
// recipient. Called from DeleteUser so that grants don't reference a missing
// user. (Grants given BY the user are handled transitively via campaign
// deletion in DeleteUser.)
func DeleteCampaignSharesByUser(viewerUid int64) error {
	return db.Where("user_id = ?", viewerUid).Delete(&CampaignShare{}).Error
}
