package models

import (
	"strings"
	"fmt"
)

// subjectRow는 JOIN 쿼리 결과를 담는 내부 구조체입니다.
type subjectRow struct {
	RId            string
	CampaignId     int64
	UserId         int64
	Email          string
	Name           string
	Department     string
	Position       string
	TemplateSubject string
}

// FindResultByEmailAndRenderedSubject는 수신자 이메일과 메일 제목으로
// 해당 Result를 찾습니다.
//
// 개선 전: result 1건당 campaign + template 쿼리 → 최대 101번 쿼리
// 개선 후: 단일 JOIN 쿼리로 subject 후보 조회 → 최대 2번 쿼리
func FindResultByEmailAndRenderedSubject(recipientEmail, submittedSubject string) (*Result, error) {
	email := strings.TrimSpace(recipientEmail)
	want := strings.TrimSpace(submittedSubject)
	if email == "" || want == "" {
		return nil, fmt.Errorf("empty email or subject")
	}

	// 1) 단일 JOIN 쿼리: results + campaigns + templates를 한 번에 조회
	//    subject에 템플릿 변수가 없는 경우 여기서 바로 매칭됩니다.
	var rows []subjectRow
	err := db.Raw(`
		SELECT
			r.r_id            AS r_id,
			r.campaign_id     AS campaign_id,
			r.user_id         AS user_id,
			r.email           AS email,
			r.name            AS name,
			r.department      AS department,
			r.position        AS position,
			t.subject         AS template_subject
		FROM results r
		INNER JOIN campaigns c  ON c.id = r.campaign_id
		INNER JOIN templates t  ON t.id = c.template_id
		WHERE r.email = ?
		ORDER BY r.send_date DESC
		LIMIT 50
	`, email).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no match")
	}

	// 2) subject에 템플릿 변수가 없으면 단순 문자열 비교로 즉시 반환
	//    변수가 있으면 렌더링 후 비교 (추가 쿼리 없음 — 이미 필요한 필드 보유)
	for _, row := range rows {
		subj := strings.TrimSpace(row.TemplateSubject)

		var matched bool
		if strings.Contains(subj, "{{") {
			// 템플릿 변수 포함 → 렌더링 후 비교
			base := BaseRecipient{
				Email:      row.Email,
				Name:       row.Name,
				Department: row.Department,
				Position:   row.Position,
			}
			// campaign 객체는 RId와 BaseRecipient 컨텍스트 생성에만 필요
			// 이미 JOIN으로 필요한 필드를 모두 가져왔으므로 경량 구조체 사용
			camp := Campaign{Id: row.CampaignId, UserId: row.UserId}
			ptx, err := NewPhishingTemplateContext(&camp, base, row.RId)
			if err != nil {
				continue
			}
			rendered, err := ExecuteTemplate(subj, ptx)
			if err != nil {
				continue
			}
			matched = strings.EqualFold(strings.TrimSpace(rendered), want)
		} else {
			// 템플릿 변수 없음 → 직접 비교
			matched = strings.EqualFold(subj, want)
		}

		if matched {
			// 매칭된 Result를 DB에서 정식으로 한 번 더 조회
			// (Result 전체 필드 + BaseRecipient 포함)
			rs, err := GetResultByRID(row.RId)
			if err != nil || rs == nil {
				continue
			}
			return rs, nil
		}
	}

	return nil, fmt.Errorf("no match")
}