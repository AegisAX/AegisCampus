package models

import (
	"fmt"
	"strings"
)

// subjectRow는 JOIN 쿼리 결과를 담는 내부 구조체입니다.
type subjectRow struct {
	RId             string
	CampaignId      int64
	UserId          int64
	Email           string
	Name            string
	Department      string
	Position        string
	TemplateSubject string
	CampaignURL     string // {{.URL}} 렌더링을 위해 추가
	SMTPFromAddress string // {{.From}} 렌더링을 위해 추가
}

// getFromAddress는 TemplateContext 인터페이스를 만족시킵니다.
func (s *subjectRow) getFromAddress() string {
	if s.SMTPFromAddress == "" {
		return "noreply@example.com" // 파싱 오류 방지용 fallback
	}
	return s.SMTPFromAddress
}

// getBaseURL은 TemplateContext 인터페이스를 만족시킵니다.
func (s *subjectRow) getBaseURL() string {
	return s.CampaignURL
}

// FindResultByEmailAndRenderedSubject는 수신자 이메일과 메일 제목으로
// 해당 Result를 찾습니다.
func FindResultByEmailAndRenderedSubject(recipientEmail, submittedSubject string) ([]*Result, error) {
	email := strings.TrimSpace(recipientEmail)
	want := strings.TrimSpace(submittedSubject)
	if email == "" || want == "" {
		return nil, fmt.Errorf("empty email or subject")
	}

	// 단일 JOIN 쿼리: SMTP FromAddress와 Campaign URL까지 함께 조회
	var rows []subjectRow
	err := db.Raw(`
		SELECT
			r.r_id              AS r_id,
			r.campaign_id       AS campaign_id,
			r.user_id           AS user_id,
			r.email             AS email,
			r.name              AS name,
			r.department        AS department,
			r.position          AS position,
			t.subject           AS template_subject,
			c.url               AS campaign_url,
			s.from_address      AS smtp_from_address
		FROM results r
		INNER JOIN campaigns c ON c.id = r.campaign_id
		INNER JOIN templates t ON t.id = c.template_id
		LEFT  JOIN smtp s      ON s.id = c.smtp_id
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

	var results []*Result

	for _, row := range rows {
		subj := strings.TrimSpace(row.TemplateSubject)

		var matched bool
		if strings.Contains(subj, "{{") {
			// 템플릿 변수 포함 → subjectRow 자체가 TemplateContext를 구현하므로
			// 경량 Campaign 구조체 불필요 — URL/FromAddress 포함 렌더링 가능
			rowCopy := row // 루프 변수 캡처 방지
			base := BaseRecipient{
				Email:      row.Email,
				Name:       row.Name,
				Department: row.Department,
				Position:   row.Position,
			}
			ptx, err := NewPhishingTemplateContext(&rowCopy, base, row.RId)
			if err != nil {
				continue
			}
			rendered, err := ExecuteTemplate(subj, ptx)
			if err != nil {
				continue
			}
			matched = strings.EqualFold(strings.TrimSpace(rendered), want)
		} else {
			matched = strings.EqualFold(subj, want)
		}

		if matched {
			rs, err := GetResultByRID(row.RId)
			if err != nil || rs == nil {
				continue
			}
			results = append(results, rs)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no match")
	}

	return results, nil
}
