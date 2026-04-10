package api

import (
    "encoding/json"
    "fmt"
    "net/http"
    "os"
    "path/filepath"
    "strconv"
    "strings"

    ctx "github.com/gophish/gophish/context"
    log "github.com/gophish/gophish/logger"
    "github.com/gophish/gophish/models"
    "github.com/gorilla/mux"
	"github.com/gophish/gophish/util"
)

// 현재 사용자 ID 추출 (없으면 0)
func getCurrentUserID(r *http.Request) int64 {
    if v := ctx.Get(r, "user_id"); v != nil {
        if vv, ok := v.(int64); ok {
            return vv
        }
    }
    return 0
}

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
    idStr := mux.Vars(r)["id"]
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil || id <= 0 {
        JSONResponse(w, models.Response{Success: false, Message: "invalid id"}, http.StatusBadRequest)
        return
    }

    switch r.Method {
    case http.MethodGet:
        v, err := models.GetVideo(id)
        if err != nil {
            if err == models.ErrVideoNotFound {
                JSONResponse(w, models.Response{Success: false, Message: "video not found"}, http.StatusNotFound)
            } else {
                JSONResponse(w, models.Response{Success: false, Message: "lookup error"}, http.StatusInternalServerError)
            }
            return
        }
        JSONResponse(w, v, http.StatusOK)

    case http.MethodPut:
        var v models.Video
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
        currentUserID := getCurrentUserID(r)
        fileDeleted, thumbDeleted, refCount, derr := deleteVideoAndFiles(id, currentUserID)
        if derr != nil {
            if derr == models.ErrVideoNotFound {
                JSONResponse(w, models.Response{Success: false, Message: "video not found"}, http.StatusNotFound)
            } else if derr.Error() == "forbidden" {
                JSONResponse(w, models.Response{Success: false, Message: "forbidden"}, http.StatusForbidden)
            } else {
                JSONResponse(w, models.Response{Success: false, Message: "delete failed"}, http.StatusInternalServerError)
            }
            return
        }
        JSONResponse(w, map[string]interface{}{
            "success":        true,
            "file_deleted":   fileDeleted,
            "thumb_deleted":  thumbDeleted,
            "file_ref_count": refCount,
        }, http.StatusOK)

    default:
        JSONResponse(w, models.Response{Success: false, Message: "Method not allowed"}, http.StatusMethodNotAllowed)
    }
}

func (as *Server) handleVideosList(w http.ResponseWriter, r *http.Request) {
    userId := getCurrentUserID(r)
    videos, err := models.GetVideosForUser(userId)
    if err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "Error fetching videos"}, http.StatusInternalServerError)
        return
    }
    JSONResponse(w, videos, http.StatusOK)
}

func (as *Server) handleVideoUpload(w http.ResponseWriter, r *http.Request) {
    userId := getCurrentUserID(r)

    const maxMultipartMem = 32 << 20
    if err := r.ParseMultipartForm(maxMultipartMem); err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "Parse error"}, http.StatusBadRequest)
        return
    }
    file, handler, err := r.FormFile("file")
    if err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "File required"}, http.StatusBadRequest)
        return
    }
    defer file.Close()

    name := strings.TrimSpace(r.FormValue("name"))
    if name == "" && handler != nil {
        base := strings.TrimSuffix(filepath.Base(handler.Filename), filepath.Ext(handler.Filename))
        name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(base, "_", " "), "-", " "))
    }
    description := r.FormValue("description")
    isPublicStr := r.FormValue("is_public")
    isPublic := isPublicStr == "1" || isPublicStr == "true"

    durationHint := int64(0)
    if ds := r.FormValue("duration_seconds"); ds != "" {
        if fv, err := strconv.ParseFloat(ds, 64); err == nil && fv >= 0 {
            durationHint = int64(fv + 0.5)
        }
    }

    originalFilename := ""
    if handler != nil {
        originalFilename = handler.Filename
    }

    result, err := util.ProcessVideoUpload(file, originalFilename, durationHint, util.VideoUploadOptions{
        IsPublic: isPublic,
    })
    if err != nil {
        log.Error(err)
        JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
        return
    }

    v := &models.Video{
        UserId:          userId,
        Name:            name,
        Description:     description,
        FileName:        result.FinalName,
        FilePath:        result.FinalPath,
        ThumbnailPath:   result.ThumbnailPath,
        DurationSeconds: result.DurationSeconds,
        IsPublic:        result.IsPublic,
    }
    if err := models.CreateVideo(v); err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "DB save error"}, http.StatusInternalServerError)
        return
    }
    JSONResponse(w, v, http.StatusCreated)
}

// /videos/thumb/{id} : 썸네일 이미지 서빙
func (as *Server) HandleVideoThumb(w http.ResponseWriter, r *http.Request) {
    idStr := mux.Vars(r)["id"]
    id, err := strconv.ParseInt(idStr, 10, 64)
    if err != nil || id <= 0 {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }
    v, err := models.GetVideo(id)
    if err != nil {
        http.NotFound(w, r)
        return
    }
    if v.ThumbnailPath == "" || !util.IsUnderBaseDir(util.VideoStorageDirAbs, v.ThumbnailPath) {
        http.NotFound(w, r)
        return
    }
    if _, err := os.Stat(v.ThumbnailPath); err != nil {
        http.NotFound(w, r)
        return
    }
    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Cache-Control", "public, max-age=86400")
    http.ServeFile(w, r, v.ThumbnailPath)
}

func deleteVideoAndFiles(id int64, currentUserID int64) (fileDeleted, thumbDeleted bool, refCount int64, err error) {
    // 1) 조회 (+ 사용자 소유 확인)
    v, err := models.GetVideo(id)
    if err != nil {
        return false, false, 0, err
    }
    if currentUserID != 0 && v.UserId != currentUserID {
        return false, false, 0, fmt.Errorf("forbidden")
    }

    // 2) 동일 파일 참조 카운트
    refCount, err = models.CountVideosByFileName(v.FileName)
    if err != nil {
        return false, false, 0, err
    }

    // 3) 실제 파일/썸네일 삭제 (참조 1개일 때만)
    if refCount <= 1 {
        if v.FilePath != "" && util.IsUnderBaseDir(util.VideoStorageDirAbs, v.FilePath) {
            if err := os.Remove(v.FilePath); err == nil {
                fileDeleted = true
            } else if !os.IsNotExist(err) {
                log.Errorf("remove video file failed: %v", err)
            }
        }
        if v.ThumbnailPath != "" && util.IsUnderBaseDir(util.VideoStorageDirAbs, v.ThumbnailPath) {
            if err := os.Remove(v.ThumbnailPath); err == nil {
                thumbDeleted = true
            } else if !os.IsNotExist(err) {
                log.Errorf("remove thumbnail failed: %v", err)
            }
        }
    } else {
        log.Infof("skip file removal: %s has %d refs", v.FileName, refCount)
    }

    // 4) DB 삭제
    if err := models.DeleteVideo(id); err != nil {
        return fileDeleted, thumbDeleted, refCount, err
    }

    log.Infof("delete video id=%d file=%s ref=%d fileDeleted=%v thumbDeleted=%v",
        id, v.FileName, refCount, fileDeleted, thumbDeleted)

    return fileDeleted, thumbDeleted, refCount, nil
}
