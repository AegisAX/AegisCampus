package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
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
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, pages, http.StatusOK)

	case http.MethodPost:
		rp := models.RedirectPage{}
		if err := json.NewDecoder(r.Body).Decode(&rp); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		// 이름 중복 검사
		_, err := models.GetRedirectPageByName(rp.Name, uid)
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: models.ErrRedirectPageNameInUse.Error()}, http.StatusConflict)
			return
		}
		rp.ModifiedDate = time.Now().UTC()
		rp.UserId = uid
		if err := models.PostRedirectPage(&rp); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
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
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 0, 64)

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
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		if newRP.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "/:id and body id mismatch"}, http.StatusBadRequest)
			return
		}
		// 이름 중복 검사: 동일 사용자의 다른 페이지와 이름이 겹치는지 확인
		existing, err := models.GetRedirectPageByName(newRP.Name, uid)
		if err == nil && existing.Id != newRP.Id {
			JSONResponse(w, models.Response{Success: false, Message: models.ErrRedirectPageNameInUse.Error()}, http.StatusConflict)
			return
		}
		newRP.ModifiedDate = time.Now().UTC()
		newRP.UserId = uid
		if err := models.PutRedirectPage(&newRP); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating redirect page: " + err.Error()}, http.StatusInternalServerError)
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
