package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/AegisAX/Sentinel/context"
	log "github.com/AegisAX/Sentinel/logger"
	"github.com/AegisAX/Sentinel/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if c.Status == models.CampaignInProgress {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
//
// (#64) GET 은 viewer(공유받은 사용자)도 통과. DELETE 는 기존대로 owner-only.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	viewerUid := ctx.Get(r, "user_id").(int64)

	switch r.Method {
	case "GET":
		allowed, ownerUid, err := models.CanViewCampaign(id, viewerUid)
		if err != nil || !allowed {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			return
		}
		c, err := models.GetCampaign(id, ownerUid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			return
		}
		JSONResponse(w, c, http.StatusOK)
	case "DELETE":
		// DELETE 는 owner-only 그대로. viewer 의 uid 로 GetCampaign 이 실패하면 404.
		if _, err := models.GetCampaign(id, viewerUid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			return
		}
		if err := models.DeleteCampaign(id); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
//
// (#64) viewer 도 결과 화면을 볼 수 있도록 게이트만 viewer 로 검사하고,
// 데이터는 owner uid 로 조회한다(GetCampaignResults 내부의 user_id 필터를
// 만족시키기 위함). 프런트가 viewer/owner 모드를 분기할 수 있도록 응답에
// is_owner 한 줄을 동봉한다.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	viewerUid := ctx.Get(r, "user_id").(int64)

	allowed, ownerUid, err := models.CanViewCampaign(id, viewerUid)
	if err != nil || !allowed {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	cr, err := models.GetCampaignResults(id, ownerUid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		// CampaignResults 구조체에 IsOwner 필드를 추가하지 않고, 응답 단계에서
		// map 으로 한 키만 덧붙여 보낸다 (모델 시그니처 영향 최소화).
		payload := map[string]interface{}{
			"id":       cr.Id,
			"name":     cr.Name,
			"status":   cr.Status,
			"results":  cr.Results,
			"timeline": cr.Events,
			"is_owner": ownerUid == viewerUid,
		}
		JSONResponse(w, payload, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
//
// (#64) GetCampaignSummary 는 내부적으로 (id, uid) 로 캠페인을 찾으므로 owner uid 로 조회.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	viewerUid := ctx.Get(r, "user_id").(int64)

	switch {
	case r.Method == "GET":
		allowed, ownerUid, err := models.CanViewCampaign(id, viewerUid)
		if err != nil || !allowed {
			JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			return
		}
		cs, err := models.GetCampaignSummary(id, ownerUid)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
//
// (#64) Complete 는 owner-only 그대로. CompleteCampaign 내부 GetCampaign(id, uid)
// 가 viewer 의 uid 로 호출되면 자동 실패한다.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}

// DELETE /api/campaigns/:id/results/:rid/report
// reported 상태를 false로 되돌림
//
// (#64) 신고 토글은 owner-only 그대로. viewer uid 로 GetCampaign 호출 → 404.
func (as *Server) CampaignResultUnreport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cid, _ := strconv.ParseInt(vars["id"], 10, 64)
	rid := vars["rid"]
	uid := ctx.Get(r, "user_id").(int64)

	// 캠페인 소유권 확인
	c, err := models.GetCampaign(cid, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	_ = c

	err = models.UnreportResult(rid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Reported status cleared"}, http.StatusOK)
}

// GET /api/campaigns/{id}/video_progress
// 캠페인 수신자별 수강 현황 조회
//
// (#64) viewer 허용. 데이터는 owner uid 로 조회.
func (as *Server) CampaignVideoProgress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cid, _ := strconv.ParseInt(vars["id"], 10, 64)
	viewerUid := ctx.Get(r, "user_id").(int64)

	allowed, ownerUid, err := models.CanViewCampaign(cid, viewerUid)
	if err != nil || !allowed {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	items, err := models.GetVideoProgressByCampaign(ownerUid, cid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, items, http.StatusOK)
}

// GET /api/campaigns/{id}/country_stats
// 캠페인 결과의 국가별 접속 Top 10
//
// (#64) viewer 허용. 데이터는 owner uid 로 조회.
func (as *Server) CampaignCountryStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cid, _ := strconv.ParseInt(vars["id"], 10, 64)
	viewerUid := ctx.Get(r, "user_id").(int64)

	allowed, ownerUid, err := models.CanViewCampaign(cid, viewerUid)
	if err != nil || !allowed {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	stats, err := models.GetCountryStatsByCampaign(ownerUid, cid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, stats, http.StatusOK)
}

// =============================================================================
// (#64) Campaign read-only sharing — owner-only management API.
//
//   GET    /api/campaigns/{id}/shares          현재 공유자 + 공유 후보 사용자
//   POST   /api/campaigns/{id}/shares          {"user_id": N} 으로 grant 추가
//   DELETE /api/campaigns/{id}/shares/{uid}    grant 해제
//
// 모두 owner-only. 캠페인 소유권은 GetCampaign(cid, uid) owner-only 검증으로
// 확인한다 (CanViewCampaign 가 아닌 GetCampaign 인 점이 핵심 — viewer 가
// 이 API 를 호출하면 404 로 차단되어야 한다).
// =============================================================================

// campaignShareUser is the public view of a user shown in the share picker.
// 민감 정보 (api_key / hash / 권한 등) 는 절대 노출하지 않는다.
type campaignShareUser struct {
	Id       int64  `json:"id"`
	Username string `json:"username"`
}

// campaignSharesResponse is returned by GET /api/campaigns/{id}/shares.
//   - Shares     : 현재 grant 가 부여된 사용자들 (id + username)
//   - Candidates : 아직 grant 가 없는 사용자 목록 (owner 본인 제외)
type campaignSharesResponse struct {
	Shares     []campaignShareUser `json:"shares"`
	Candidates []campaignShareUser `json:"candidates"`
}

// requireCampaignOwner verifies that the requester owns the campaign with the
// given id, and writes a 404 + returns false if not. Used by all share
// management endpoints.
func (as *Server) requireCampaignOwner(w http.ResponseWriter, r *http.Request, cid int64) bool {
	uid := ctx.Get(r, "user_id").(int64)
	if _, err := models.GetCampaign(cid, uid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return false
	}
	return true
}

// CampaignShares handles GET and POST on /api/campaigns/{id}/shares.
func (as *Server) CampaignShares(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cid, _ := strconv.ParseInt(vars["id"], 10, 64)
	if !as.requireCampaignOwner(w, r, cid) {
		return
	}
	ownerUid := ctx.Get(r, "user_id").(int64)

	switch r.Method {
	case "GET":
		// 현재 공유 목록
		shares, err := models.GetCampaignShares(cid)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		sharedSet := map[int64]bool{}
		for _, s := range shares {
			sharedSet[s.UserId] = true
		}
		// 전체 사용자에서 owner + 이미 공유된 사람 제외 → 후보
		allUsers, err := models.GetUsers()
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		resp := campaignSharesResponse{
			Shares:     []campaignShareUser{},
			Candidates: []campaignShareUser{},
		}
		userById := map[int64]string{}
		for _, u := range allUsers {
			userById[u.Id] = u.Username
			if u.Id == ownerUid {
				continue
			}
			if sharedSet[u.Id] {
				continue
			}
			resp.Candidates = append(resp.Candidates, campaignShareUser{Id: u.Id, Username: u.Username})
		}
		// shares 는 created_date 기록을 살리되, 사용자명을 채워서 응답.
		// 사용자 행이 사라진 경우(이론상 cascade 로 정리됨)엔 빈 문자열.
		for _, s := range shares {
			resp.Shares = append(resp.Shares, campaignShareUser{
				Id:       s.UserId,
				Username: userById[s.UserId],
			})
		}
		JSONResponse(w, resp, http.StatusOK)

	case "POST":
		var body struct {
			UserId int64 `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserId <= 0 {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request body"}, http.StatusBadRequest)
			return
		}
		// 자기 자신에게 공유 금지 (의미 없음)
		if body.UserId == ownerUid {
			JSONResponse(w, models.Response{Success: false, Message: "Cannot share with the campaign owner"}, http.StatusBadRequest)
			return
		}
		// 존재하는 사용자만 grant 허용
		if _, err := models.GetUser(body.UserId); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "User not found"}, http.StatusNotFound)
			return
		}
		if err := models.AddCampaignShare(cid, body.UserId); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Share added"}, http.StatusCreated)
	}
}

// CampaignShareDelete handles DELETE /api/campaigns/{id}/shares/{uid}.
func (as *Server) CampaignShareDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cid, _ := strconv.ParseInt(vars["id"], 10, 64)
	if !as.requireCampaignOwner(w, r, cid) {
		return
	}
	targetUid, err := strconv.ParseInt(vars["uid"], 10, 64)
	if err != nil || targetUid <= 0 {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid user id"}, http.StatusBadRequest)
		return
	}
	if err := models.DeleteCampaignShare(cid, targetUid); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: "Share removed"}, http.StatusOK)
}
