package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/AegisAX/AegisCampus/context"
	log "github.com/AegisAX/AegisCampus/logger"
	"github.com/AegisAX/AegisCampus/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// RedirectPages handles GET /api/redirect_pages/ and POST /api/redirect_pages/
func (as *Server) RedirectPages(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch r.Method {
	case http.MethodGet:
		pages, err := models.GetRedirectPages(uid)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Error retrieving redirect pages"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, pages, http.StatusOK)

	case http.MethodPost:
		rp := models.RedirectPage{}
		if err := json.NewDecoder(r.Body).Decode(&rp); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		_, err := models.GetRedirectPageByName(rp.Name, uid)
		if err == nil { // 이름 중복
			JSONResponse(w, models.Response{Success: false, Message: models.ErrRedirectPageNameInUse.Error()}, http.StatusConflict)
			return
		} else if err != gorm.ErrRecordNotFound { // DB 오류
			JSONResponse(w, models.Response{Success: false, Message: "Error checking redirect page name"}, http.StatusInternalServerError)
			log.Error(err)
			return
		}
		rp.UserId = uid
		// (#38) 비디오 임베드 권한 검사 — 자기 영상 또는 IsPublic 영상만 허용
		if rp.VideoId != nil && *rp.VideoId > 0 {
			can, err := models.CanUserUseVideo(*rp.VideoId, uid)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: "Error checking video access"}, http.StatusInternalServerError)
				log.Error(err)
				return
			}
			if !can {
				JSONResponse(w, models.Response{Success: false, Message: "선택한 동영상에 접근 권한이 없습니다."}, http.StatusForbidden)
				return
			}
		}
		if verr := models.ValidateTemplate(rp.HTML); verr != nil {
			JSONResponse(w, models.Response{Success: false, Message: verr.Error()}, http.StatusBadRequest)
			return
		}
		if verr := models.ValidateTemplate(rp.RedirectURL); verr != nil {
			JSONResponse(w, models.Response{Success: false, Message: verr.Error()}, http.StatusBadRequest)
			return
		}
		err = models.PostRedirectPage(&rp)
		if err == models.ErrRedirectPageNameNotSpecified {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error inserting redirect page into database"}, http.StatusInternalServerError)
			log.Error(err)
			return
		}
		JSONResponse(w, rp, http.StatusCreated)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// RedirectPage handles GET/PUT/DELETE /api/redirect_pages/{id}
func (as *Server) RedirectPage(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil || id <= 0 {
		JSONResponse(w, models.Response{Success: false, Message: "invalid id"}, http.StatusBadRequest)
		return
	}

	rp, err := models.GetRedirectPage(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Redirect Page not found"}, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		JSONResponse(w, rp, http.StatusOK)

	case http.MethodPut:
		newRP := models.RedirectPage{}
		if err := json.NewDecoder(r.Body).Decode(&newRP); err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		newRP.Id = id
		// 이름 중복 검사: 동일 사용자의 다른 페이지와 이름이 겹치는지 확인
		existing, err := models.GetRedirectPageByName(newRP.Name, uid)
		if err == nil && existing.Id != newRP.Id {
			JSONResponse(w, models.Response{Success: false, Message: models.ErrRedirectPageNameInUse.Error()}, http.StatusConflict)
			return
		}
		newRP.UserId = uid
		// (#38) 비디오 임베드 권한 검사 — 자기 영상 또는 IsPublic 영상만 허용
		if newRP.VideoId != nil && *newRP.VideoId > 0 {
			can, err := models.CanUserUseVideo(*newRP.VideoId, uid)
			if err != nil {
				JSONResponse(w, models.Response{Success: false, Message: "Error checking video access"}, http.StatusInternalServerError)
				log.Error(err)
				return
			}
			if !can {
				JSONResponse(w, models.Response{Success: false, Message: "선택한 동영상에 접근 권한이 없습니다."}, http.StatusForbidden)
				return
			}
		}
		if verr := models.ValidateTemplate(newRP.HTML); verr != nil {
			JSONResponse(w, models.Response{Success: false, Message: verr.Error()}, http.StatusBadRequest)
			return
		}
		if verr := models.ValidateTemplate(newRP.RedirectURL); verr != nil {
			JSONResponse(w, models.Response{Success: false, Message: verr.Error()}, http.StatusBadRequest)
			return
		}
		err = models.PutRedirectPage(&newRP)
		if err == models.ErrRedirectPageNameNotSpecified {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating redirect page in database"}, http.StatusInternalServerError)
			log.Error(err)
			return
		}
		JSONResponse(w, newRP, http.StatusOK)

	case http.MethodDelete:
		if err := models.DeleteRedirectPage(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting redirect page"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Redirect Page deleted successfully"}, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
	}
}
