package api

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
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

    // 디스크 스필 기준 메모리 한도
    const maxMultipartMem = 32 << 20 // 32MB
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

    if err := os.MkdirAll(util.VideoStorageDirAbs, 0755); err != nil {
        log.Error(err)
        JSONResponse(w, models.Response{Success: false, Message: "Storage error"}, http.StatusInternalServerError)
        return
    }
    if err := os.MkdirAll(util.VideoThumbDirAbs, 0755); err != nil {
        log.Error(err)
        JSONResponse(w, models.Response{Success: false, Message: "Thumb storage error"}, http.StatusInternalServerError)
        return
    }

    // 1) 임시 파일에 쓰면서 sha256 해시 동시 계산
    tmpFile, err := os.CreateTemp(util.VideoStorageDirAbs, "upload-*")
    if err != nil {
        log.Error(err)
        JSONResponse(w, models.Response{Success: false, Message: "Create temp file error"}, http.StatusInternalServerError)
        return
    }
    tmpName := tmpFile.Name()
    cleanupTmp := true
    defer func() {
        if cleanupTmp {
            _ = os.Remove(tmpName)
        }
    }()

    hasher := sha256.New()
    if _, err := io.Copy(io.MultiWriter(tmpFile, hasher), file); err != nil {
        _ = tmpFile.Close()
        JSONResponse(w, models.Response{Success: false, Message: "Write file error"}, http.StatusInternalServerError)
        return
    }
    if err := tmpFile.Close(); err != nil {
        JSONResponse(w, models.Response{Success: false, Message: "Temp file close error"}, http.StatusInternalServerError)
        return
    }

    // 2) 최종 파일명 = <sha256hex><원본 확장자>
    sumHex := hex.EncodeToString(hasher.Sum(nil))
    ext := ""
    if handler != nil {
        ext = strings.ToLower(filepath.Ext(handler.Filename)) // 없으면 "" 유지
    }
    finalName := sumHex + ext
    finalPath := filepath.Join(util.VideoStorageDirAbs, finalName)

    // 3) 중복 파일 처리: 이미 있으면 임시파일 삭제, 없으면 rename
    if _, err := os.Stat(finalPath); err == nil {
        // 동일 해시 파일 존재 → 임시파일 제거 후 재사용
        cleanupTmp = true
    } else {
        if err := os.Rename(tmpName, finalPath); err != nil {
            // 같은 디렉터리라면 일반적으로 성공. 혹시 실패 시 복사 fallback
            in, err1 := os.Open(tmpName)
            if err1 != nil {
                JSONResponse(w, models.Response{Success: false, Message: "Finalize file error"}, http.StatusInternalServerError)
                return
            }
            out, err2 := os.Create(finalPath)
            if err2 != nil {
                in.Close()
                JSONResponse(w, models.Response{Success: false, Message: "Finalize file error"}, http.StatusInternalServerError)
                return
            }
            if _, err := io.Copy(out, in); err != nil {
                out.Close()
                in.Close()
                _ = os.Remove(finalPath)
                JSONResponse(w, models.Response{Success: false, Message: "Finalize file error"}, http.StatusInternalServerError)
                return
            }
            out.Close()
            in.Close()
        } else {
            cleanupTmp = false // rename 성공 → tmp 없음
        }
    }

    // 4) 길이 계산
    durationSeconds := int64(0)
    if ds := r.FormValue("duration_seconds"); ds != "" {
        if v, err := strconv.ParseFloat(ds, 64); err == nil && v >= 0 {
            durationSeconds = int64(v + 0.5)
        }
    }
    if durationSeconds == 0 {
        if d, err := util.ProbeDurationSeconds(finalPath); err == nil && d > 0 {
            durationSeconds = d
        }
    }

    // 5) 썸네일 생성 (ffmpeg)
    thumbName := sumHex + ".jpg"
    thumbPath := filepath.Join(util.VideoThumbDirAbs, thumbName)
    // 썸네일 시점: 1초(또는 총 길이-1 중 더 작은 값)
    at := 1
    if durationSeconds > 2 {
        if durationSeconds-1 < 3 {
            at = int(durationSeconds - 1)
        } else {
            at = 3
        }
    }
    if err := util.GenerateThumbnail(finalPath, thumbPath, at, 320); err != nil {
        // 썸네일 실패는 치명적이지 않음 → 로그만 남김
        log.Errorf("thumbnail generation failed: %v", err)
        thumbPath = "" // 빈 값 저장
    }

    v := &models.Video{
        UserId:          userId,
        Name:            name,
        Description:     description,
        FileName:        finalName,
        FilePath:        finalPath,
        ThumbnailPath:   thumbPath,
        DurationSeconds: durationSeconds,
        IsPublic:        isPublic,
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
