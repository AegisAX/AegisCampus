package api

import (
	"encoding/json"
	"net/http"

	ctx "github.com/AegisAX/Sentinel/context"
	log "github.com/AegisAX/Sentinel/logger"
	"github.com/AegisAX/Sentinel/models"
)

// UserPreferencesMe handles GET / PUT on /api/users/me/preferences.
//
// (#66) 본인 사용자의 환경설정을 조회/저장한다. 라우트가 "me" 라 별도 RBAC
// 불필요 — RequireAPIKey 미들웨어가 user_id 를 컨텍스트에 박아주고, 본 핸들러는
// 그 user_id 로만 조회/저장한다.
//
// 향후 다른 preference 키가 늘면 본문 JSON 에 키만 추가하면 되도록 일반화된
// 엔드포인트로 설계.
func (as *Server) UserPreferencesMe(w http.ResponseWriter, r *http.Request) {
	uid, ok := ctx.Get(r, "user_id").(int64)
	if !ok || uid == 0 {
		JSONResponse(w, models.Response{Success: false, Message: "Unauthorized"}, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case "GET":
		p, err := models.GetUserPreferences(uid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, p, http.StatusOK)

	case "PUT":
		var body struct {
			DashboardCampaignFilter string `json:"dashboard_campaign_filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request body"}, http.StatusBadRequest)
			return
		}
		p := &models.UserPreferences{
			UserId:                  uid,
			DashboardCampaignFilter: body.DashboardCampaignFilter,
		}
		if err := models.SaveUserPreferences(p); err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, p, http.StatusOK)
	}
}
