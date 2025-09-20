package api

import (
    "encoding/json"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "time"

    ctx "github.com/gophish/gophish/context"
    log "github.com/gophish/gophish/logger"
    "github.com/gophish/gophish/models"
    "github.com/gorilla/mux"
)

var videoStorageDir = "static/videos"

// 메서드명은 명확히: HandleVideos, HandleVideoByID
func (as *Server) HandleVideos(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        as.handleVideosList(w, r)
    case http.MethodPost:
        as.handleVideoUpload(w, r)
    default:
        JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
    }
}

func (as *Server) HandleVideoByID(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    idStr := vars["id"]
    id, _ := strconv.ParseInt(idStr, 10, 64)

    switch r.Method {
    case http.MethodGet:
        v, err := models.GetVideo(id)
        if err != nil {
            JSONResponse(w, models.Response{Success: false, Message: "Not found"}, http.StatusNotFound)
            return
        }
        JSONResponse(w, v, http.StatusOK)
    case http.MethodPut:
        v := models.Video{}
        if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
            JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
            return
        }
        v.Id = id
        if err := models.UpdateVideo(&v); err != nil {
            JSONResponse(w, models.Response{Success: false, Message: "Update failed"}, http.StatusInternalServerError)
            return
        }
        JSONResponse(w, models.Response{Success: true}, http.StatusOK)
    case http.MethodDelete:
        if err := models.DeleteVideo(id); err != nil {
            JSONResponse(w, models.Response{Success: false, Message: "Delete failed"}, http.StatusInternalServerError)
            return
        }
        JSONResponse(w, models.Response{Success: true}, http.StatusOK)
    default:
        JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
    }
}

func (as *Server) handleVideosList(w http.ResponseWriter, r *http.Request) {
    userId := int64(0)
    if v := ctx.Get(r, "user_id"); v != nil {
        if vv, ok := v.(int64); ok {
            userId = vv
        }
    }
    videos, err := models.GetVideosForUser(userId)
    if err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "Error fetching videos"}, http.StatusInternalServerError)
        return
    }
    JSONResponse(w, videos, http.StatusOK)
}

func (as *Server) handleVideoUpload(w http.ResponseWriter, r *http.Request) {
    userId := int64(0)
    if v := ctx.Get(r, "user_id"); v != nil {
        if vv, ok := v.(int64); ok {
            userId = vv
        }
    }

    if err := r.ParseMultipartForm(1 << 30); err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "Parse error"}, http.StatusBadRequest)
        return
    }
    file, handler, err := r.FormFile("file")
    if err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "File required"}, http.StatusBadRequest)
        return
    }
    defer file.Close()

    name := r.FormValue("name")
    description := r.FormValue("description")
    isPublicStr := r.FormValue("is_public")
    isPublic := isPublicStr == "1" || isPublicStr == "true"

    if err := os.MkdirAll(videoStorageDir, 0755); err != nil {
        log.Error(err)
        JSONResponse(w, models.Response{Success: false, Message: "Storage error"}, http.StatusInternalServerError)
        return
    }

    outName := strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + handler.Filename
    outPath := filepath.Join(videoStorageDir, outName)
    outFile, err := os.Create(outPath)
    if err != nil {
        log.Error(err)
        JSONResponse(w, models.Response{Success: false, Message: "Create file error"}, http.StatusInternalServerError)
        return
    }
    if _, err := io.Copy(outFile, file); err != nil {
        outFile.Close()
        JSONResponse(w, models.Response{Success: false, Message: "Write file error"}, http.StatusInternalServerError)
        return
    }
    outFile.Close()

    v := &models.Video{
        UserId:          userId,
        Name:            name,
        Description:     description,
        FileName:        handler.Filename,
        FilePath:        outPath,
        ThumbnailPath:   "",
        DurationSeconds: 0,
        IsPublic:        isPublic,
    }
    if err := models.CreateVideo(v); err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "DB save error"}, http.StatusInternalServerError)
        return
    }
    JSONResponse(w, v, http.StatusCreated)
}

