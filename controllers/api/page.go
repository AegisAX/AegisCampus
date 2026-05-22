package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	ctx "github.com/AegisAX/Sentinel/context"
	log "github.com/AegisAX/Sentinel/logger"
	"github.com/AegisAX/Sentinel/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Pages handles requests for the /api/pages/ endpoint
func (as *Server) Pages(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ps, err := models.GetPages(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ps, http.StatusOK)
	//POST: Create a new page and return it as JSON
	case r.Method == "POST":
		p := models.Page{}
		// Put the request into a page
		err := json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		// Check to make sure the name is unique
		_, err = models.GetPageByName(p.Name, ctx.Get(r, "user_id").(int64))
		if err != gorm.ErrRecordNotFound {
			JSONResponse(w, models.Response{Success: false, Message: "Page name already in use"}, http.StatusConflict)
			return
		}
		p.ModifiedDate = time.Now().UTC()
		p.UserId = ctx.Get(r, "user_id").(int64)
		// (#38) 비디오 임베드 권한 검사 — 자기 영상 또는 IsPublic 영상만 허용
		if p.VideoId != nil && *p.VideoId > 0 {
			can, err := models.CanUserUseVideo(*p.VideoId, p.UserId)
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
		err = models.PostPage(&p)
		if err == models.ErrPageNameNotSpecified {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error inserting page into database"}, http.StatusInternalServerError)
			log.Error(err)
			return
		}
		JSONResponse(w, p, http.StatusCreated)
	}
}

// Page contains functions to handle the GET'ing, DELETE'ing, and PUT'ing
// of a Page object
func (as *Server) Page(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	p, err := models.GetPage(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Page not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, p, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeletePage(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting page"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Page Deleted Successfully"}, http.StatusOK)
	case r.Method == "PUT":
		p = models.Page{}
		err = json.NewDecoder(r.Body).Decode(&p)
		if err != nil {
			log.Error(err)
		}
		if p.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and page_id mismatch"}, http.StatusBadRequest)
			return
		}
		p.ModifiedDate = time.Now().UTC()
		p.UserId = ctx.Get(r, "user_id").(int64)
		// (#38) 비디오 임베드 권한 검사 — 자기 영상 또는 IsPublic 영상만 허용
		if p.VideoId != nil && *p.VideoId > 0 {
			can, err := models.CanUserUseVideo(*p.VideoId, p.UserId)
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
		err = models.PutPage(&p)
		if err == models.ErrPageNameNotSpecified {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error updating page in database"}, http.StatusInternalServerError)
			log.Error(err)
			return
		}
		JSONResponse(w, p, http.StatusOK)
	}
}
