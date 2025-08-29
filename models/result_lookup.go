package models

import (
        "strings"
        "fmt"
)

// FindResultByEmailAndRenderedSubject finds a Result by recipient email and the
// rendered subject (case-insensitive). It scans recent results for that email,
// renders each campaign's template subject with the recipient context, and
// returns the first subject match.
func FindResultByEmailAndRenderedSubject(recipientEmail, submittedSubject string) (*Result, error) {
        email := strings.TrimSpace(recipientEmail)
        want := strings.TrimSpace(submittedSubject)
        if email == "" || want == "" {
                return nil, fmt.Errorf("empty email or subject")
        }

        var candidates []Result
        // NOTE: db는 models 패키지 내부 전역 핸들(비공개 변수)입니다.
        // 외부(controllers)에서는 접근할 수 없으므로, 이 헬퍼가 대신 쿼리합니다.
        if err := db.
                Where("email = ?", email).
                Order("send_date DESC").
                Limit(50).
                Find(&candidates).Error; err != nil {
                return nil, err
        }

        for i := range candidates {
                rs := &candidates[i]

                // Load campaign/template
                camp, err := GetCampaign(rs.CampaignId, rs.UserId)
                if err != nil {
                        continue
                }
                tpl, err := GetTemplate(camp.TemplateId, camp.UserId)
                if err != nil {
                        continue
                }

                // Render template subject in the recipient context
                ptx, err := NewPhishingTemplateContext(&camp, rs.BaseRecipient, rs.RId)
                if err != nil {
                        continue
                }
                renderedSubj, err := ExecuteTemplate(tpl.Subject, ptx)
                if err != nil {
                        continue
                }

                if strings.EqualFold(strings.TrimSpace(renderedSubj), want) {
                        return rs, nil
                }
        }
        return nil, fmt.Errorf("no match")
}
