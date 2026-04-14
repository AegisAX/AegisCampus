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
	"github.com/gophish/gophish/util"
	"github.com/gorilla/mux"
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
		currentUserID := getCurrentUserID(r)

		// 1) 소유권 확인
		existing, err := models.GetVideo(id)
		if err != nil {
			if err == models.ErrVideoNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "video not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: "lookup error"}, http.StatusInternalServerError)
			}
			return
		}
		if existing.UserId != currentUserID {
			JSONResponse(w, models.Response{Success: false, Message: "forbidden"}, http.StatusForbidden)
			return
		}

		// 2) Redirect Page 사용 중 확인 (소유자 포함 전체)
		//    IsVideoUsedByOthers → IsVideoInUse 로 교체
		inUse, err := models.IsVideoInUse(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "lookup error"}, http.StatusInternalServerError)
			return
		}
		if inUse {
			JSONResponse(w, models.Response{Success: false, Message: "Redirect Page에서 사용 중인 동영상은 수정할 수 없습니다"}, http.StatusConflict)
			return
		}

		// 3) Content-Type에 따라 분기
		ct := r.Header.Get("Content-Type")
		if strings.Contains(ct, "multipart/form-data") {
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				JSONResponse(w, models.Response{Success: false, Message: "Parse error"}, http.StatusBadRequest)
				return
			}
			name := strings.TrimSpace(r.FormValue("name"))
			description := r.FormValue("description")
			isPublicStr := r.FormValue("is_public")
			isPublic := isPublicStr == "1" || isPublicStr == "true"

			file, handler, fileErr := r.FormFile("file")
			if fileErr == nil {
				// IsVideoInUse 체크는 위(2번)에서 이미 완료 → 중복 제거
				defer file.Close()
				durationHint := int64(0)
				if ds := r.FormValue("duration_seconds"); ds != "" {
					if fv, err := strconv.ParseFloat(ds, 64); err == nil && fv >= 0 {
						durationHint = int64(fv + 0.5)
					}
				}
				result, err := util.ProcessVideoUpload(file, handler.Filename, durationHint, util.VideoUploadOptions{
					IsPublic: isPublic,
				})
				if err != nil {
					log.Error(err)
					JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
					return
				}
				thumbRel := ""
				if result.ThumbnailPath != "" {
					thumbRel = filepath.Join("thumbs", filepath.Base(result.ThumbnailPath))
				}
				if existing.FileName != result.FinalName {
					refCount, _ := models.CountVideosByFileName(existing.FileName)
					if refCount <= 1 {
						oldAbs := existing.FilePath
						if !filepath.IsAbs(oldAbs) {
							oldAbs = filepath.Join(util.VideoStorageDirAbs, oldAbs)
						}
						_ = os.Remove(oldAbs)
						oldThumb := existing.ThumbnailPath
						if !filepath.IsAbs(oldThumb) {
							oldThumb = filepath.Join(util.VideoStorageDirAbs, oldThumb)
						}
						_ = os.Remove(oldThumb)
					}
				}
				existing.Name = name
				existing.Description = description
				existing.IsPublic = isPublic
				existing.FileName = result.FinalName
				existing.FilePath = result.FinalName
				existing.ThumbnailPath = thumbRel
				existing.DurationSeconds = result.DurationSeconds
			} else {
				existing.Name = name
				existing.Description = description
				existing.IsPublic = isPublic
			}
			if err := models.UpdateVideo(existing); err != nil {
				JSONResponse(w, models.Response{Success: false, Message: "Update failed"}, http.StatusInternalServerError)
				return
			}
			JSONResponse(w, existing, http.StatusOK)

		} else {
			// JSON 분기 — IsVideoInUse 체크는 위(2번)에서 완료
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
		}

	case http.MethodDelete:
		currentUserID := getCurrentUserID(r)

		// Redirect Page 사용 중 확인
		inUse, err := models.IsVideoInUse(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "lookup error"}, http.StatusInternalServerError)
			return
		}
		if inUse {
			JSONResponse(w, models.Response{Success: false, Message: "Redirect Page에서 사용 중인 동영상은 삭제할 수 없습니다"}, http.StatusConflict)
			return
		}

		// 다른 사용자 사용 중 확인 (소유권은 deleteVideoAndFiles 내부에서 처리)
		inUseByOthers, err := models.IsVideoUsedByOthers(id, currentUserID)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "lookup error"}, http.StatusInternalServerError)
			return
		}
		if inUseByOthers {
			JSONResponse(w, models.Response{Success: false, Message: "다른 사용자가 사용 중인 동영상은 삭제할 수 없습니다"}, http.StatusConflict)
			return
		}

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
	// is_owner 필드를 포함한 응답 구조체
	type VideoItem struct {
		models.Video
		IsOwner bool `json:"is_owner"`
	}
	items := make([]VideoItem, len(videos))
	for i, v := range videos {
		items[i] = VideoItem{Video: v, IsOwner: v.UserId == userId}
	}
	JSONResponse(w, items, http.StatusOK)
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

	// result 받은 후 저장 부분만 변경
	thumbRel := ""
	if result.ThumbnailPath != "" {
		thumbRel = filepath.Join("thumbs", filepath.Base(result.ThumbnailPath))
	}

	v := &models.Video{
		UserId:          userId,
		Name:            name,
		Description:     description,
		FileName:        result.FinalName,
		FilePath:        result.FinalName, // 파일명만 (상대경로)
		ThumbnailPath:   thumbRel,         // thumbs/xxxx.jpg (상대경로)
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
	if v.ThumbnailPath == "" {
		http.NotFound(w, r)
		return
	}
	// 절대/상대 경로 모두 처리 → 절대경로로 정규화
	thumbPath := v.ThumbnailPath
	if !filepath.IsAbs(thumbPath) {
		thumbPath = filepath.Join(util.VideoStorageDirAbs, thumbPath)
	}
	if !util.IsUnderBaseDir(util.VideoStorageDirAbs, thumbPath) {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(thumbPath); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, thumbPath)
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
		// 절대/상대 경로 모두 처리
		absFilePath := v.FilePath
		if !filepath.IsAbs(absFilePath) && absFilePath != "" {
			absFilePath = filepath.Join(util.VideoStorageDirAbs, absFilePath)
		}
		if absFilePath != "" && util.IsUnderBaseDir(util.VideoStorageDirAbs, absFilePath) {
			if err := os.Remove(absFilePath); err == nil {
				fileDeleted = true
			} else if !os.IsNotExist(err) {
				log.Errorf("remove video file failed: %v", err)
			}
		}

		absThumbPath := v.ThumbnailPath
		if !filepath.IsAbs(absThumbPath) && absThumbPath != "" {
			absThumbPath = filepath.Join(util.VideoStorageDirAbs, absThumbPath)
		}
		if absThumbPath != "" && util.IsUnderBaseDir(util.VideoStorageDirAbs, absThumbPath) {
			if err := os.Remove(absThumbPath); err == nil {
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
