package util

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrUnsupportedVideoExt 는 업로드 시 화이트리스트에 없는 확장자를 받았을 때 반환된다.
// 호출자는 errors.Is(err, util.ErrUnsupportedVideoExt) 로 식별하여 HTTP 415 응답에 사용한다.
var ErrUnsupportedVideoExt = errors.New("unsupported video extension")

// AllowedVideoExt 는 업로드 가능한 영상 확장자 화이트리스트 (소문자, 점 포함). (#23)
var AllowedVideoExt = map[string]bool{
	".mp4":  true,
	".webm": true,
}

// ExtToMimeType 는 확장자 → Content-Type 매핑. (#23)
var ExtToMimeType = map[string]string{
	".mp4":  "video/mp4",
	".webm": "video/webm",
}

// MimeTypeForFileName 은 파일명의 확장자를 보고 Content-Type 을 반환한다. (#23)
// 알 수 없는 확장자면 "video/mp4" fallback (DB 기존 영상 호환).
func MimeTypeForFileName(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if mime, ok := ExtToMimeType[ext]; ok {
		return mime
	}
	return "video/mp4"
}

// VideoStorageDir은 영상 파일 저장 기본 경로입니다.
var VideoStorageDir = "static/videos"

// VideoStorageDirAbs는 VideoStorageDir의 절대 경로입니다.
var VideoStorageDirAbs = func() string {
	if p, err := filepath.Abs(VideoStorageDir); err == nil {
		return p
	}
	return filepath.Clean(VideoStorageDir)
}()

// VideoThumbDirAbs는 썸네일 저장 절대 경로입니다.
var VideoThumbDirAbs = filepath.Join(VideoStorageDirAbs, "thumbs")

// FfmpegBin은 ffmpeg 바이너리 경로입니다. 환경변수 AEGISCAMPUS_FFMPEG로 오버라이드 가능합니다.
var FfmpegBin = videoEnvDefault("AEGISCAMPUS_FFMPEG", "ffmpeg")

// FfprobeBin은 ffprobe 바이너리 경로입니다. 환경변수 AEGISCAMPUS_FFPROBE로 오버라이드 가능합니다.
var FfprobeBin = videoEnvDefault("AEGISCAMPUS_FFPROBE", "ffprobe")

func videoEnvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// MaxVideoUploadBytes는 영상 업로드 요청의 최대 허용 크기(바이트)입니다.
// 환경변수 AEGISCAMPUS_MAX_VIDEO_BYTES 로 오버라이드할 수 있습니다.
// 예: export AEGISCAMPUS_MAX_VIDEO_BYTES=1073741824  # 1GB
var MaxVideoUploadBytes = videoMaxBytesDefault()

func videoMaxBytesDefault() int64 {
	if v := os.Getenv("AEGISCAMPUS_MAX_VIDEO_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			return n
		}
	}
	return 500 << 20 // 기본값 500MB
}

// init은 패키지 로드 시 ffmpeg/ffprobe 설치 여부를 확인하고
// 없으면 stderr에 경고를 출력합니다.
// Sentinel 기동 시 운영자가 바로 인지할 수 있도록 합니다.
func init() {
	checkVideoBin(FfmpegBin, "ffmpeg", "AEGISCAMPUS_FFMPEG", "동영상 썸네일 생성 및 재인코딩")
	checkVideoBin(FfprobeBin, "ffprobe", "AEGISCAMPUS_FFPROBE", "동영상 재생 시간(길이) 자동 감지")
}

func checkVideoBin(bin, name, envKey, purpose string) {
	if _, err := exec.LookPath(bin); err != nil {
		fmt.Fprintf(os.Stderr,
			"[Sentinel WARN] %s을(를) PATH에서 찾을 수 없습니다 (bin=%q).\n"+
				"  영향 기능: %s\n"+
				"  영상 업로드는 가능하지만 해당 기능이 비활성화됩니다.\n"+
				"  해결 방법: sudo apt install %s  또는  환경변수 %s=<경로> 설정\n",
			name, bin, purpose, name, envKey)
	}
}

// IsUnderBaseDir는 target 경로가 base 디렉터리 하위에 있는지 검사합니다.
// path traversal 공격 방지용으로 사용합니다.
func IsUnderBaseDir(base, target string) bool {
	base = filepath.Clean(base)
	target = filepath.Clean(target)
	if base == target {
		return true
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// ProbeDurationSeconds는 ffprobe로 영상 길이(초)를 반환합니다.
func ProbeDurationSeconds(path string) (int64, error) {
	if _, err := exec.LookPath(FfprobeBin); err != nil {
		return 0, fmt.Errorf("ffprobe not found: %w", err)
	}
	cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, FfprobeBin, "-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	)
	out, err := cmd.CombinedOutput()
	if cctx.Err() == context.DeadlineExceeded {
		return 0, fmt.Errorf("ffprobe timeout")
	}
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if f < 0 {
		return 0, fmt.Errorf("negative duration")
	}
	return int64(f + 0.5), nil
}

// GenerateThumbnail은 ffmpeg으로 영상에서 썸네일 이미지를 생성합니다.
// widthPx: 가로 최대폭 (세로는 종횡비 유지)
func GenerateThumbnail(inputPath, outputPath string, atSecond int, widthPx int) error {
	if _, err := exec.LookPath(FfmpegBin); err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}
	cctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ss := strconv.Itoa(atSecond)
	scale := fmt.Sprintf("scale=%d:-1:force_original_aspect_ratio=decrease", widthPx)
	cmd := exec.CommandContext(cctx, FfmpegBin, "-v", "error",
		"-ss", ss, "-i", inputPath,
		"-frames:v", "1",
		"-vf", scale,
		"-y", outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("ffmpeg timeout")
		}
		return fmt.Errorf("ffmpeg error: %v (%s)", err, string(out))
	}
	return nil
}

// VideoUploadOptions holds configuration for ProcessVideoUpload.
type VideoUploadOptions struct {
	IsPublic bool
}

// VideoUploadResult holds the result of a successful upload.
type VideoUploadResult struct {
	FinalName       string
	FinalPath       string
	ThumbnailPath   string
	DurationSeconds int64
	IsPublic        bool
}

// ProcessVideoUpload handles the common file-processing logic shared between
// the admin UI upload handler and the REST API upload handler.
// It reads from file (multipart.File), writes to disk with SHA-256 dedup,
// generates a thumbnail, and returns metadata ready for models.CreateVideo.
func ProcessVideoUpload(
	file io.Reader,
	originalFilename string,
	durationHint int64, // 클라이언트가 제공한 길이(초). 0이면 ffprobe로 탐지
	opts VideoUploadOptions,
) (*VideoUploadResult, error) {

	// 1) 디렉터리 준비
	if err := os.MkdirAll(VideoStorageDirAbs, 0755); err != nil {
		return nil, fmt.Errorf("storage dir: %w", err)
	}
	if err := os.MkdirAll(VideoThumbDirAbs, 0755); err != nil {
		return nil, fmt.Errorf("thumb dir: %w", err)
	}

	// 2) 임시 파일에 쓰면서 SHA-256 계산
	tmpFile, err := os.CreateTemp(VideoStorageDirAbs, "upload-*")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
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
		return nil, fmt.Errorf("write temp: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp: %w", err)
	}

	// 3) 최종 파일명 = <sha256hex><원본 확장자>
	sumHex := hex.EncodeToString(hasher.Sum(nil))
	ext := ""
	if originalFilename != "" {
		ext = strings.ToLower(filepath.Ext(originalFilename))
	}
	// (#23) 확장자 화이트리스트 검사 — Allowed 외 거부
	if !AllowedVideoExt[ext] {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedVideoExt, ext)
	}
	finalName := sumHex + ext
	finalPath := filepath.Join(VideoStorageDirAbs, finalName)

	// 4) 중복 파일 처리
	if _, err := os.Stat(finalPath); err == nil {
		// 동일 해시 이미 존재 → defer가 tmpName 정리
	} else {
		if err := os.Rename(tmpName, finalPath); err != nil {
			// rename 실패 → 복사 fallback
			in, err1 := os.Open(tmpName)
			if err1 != nil {
				return nil, fmt.Errorf("open temp for copy: %w", err1)
			}
			out, err2 := os.Create(finalPath)
			if err2 != nil {
				in.Close()
				return nil, fmt.Errorf("create final: %w", err2)
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close()
				in.Close()
				_ = os.Remove(finalPath)
				return nil, fmt.Errorf("copy to final: %w", err)
			}
			out.Close()
			in.Close()
			// 복사 완료 → defer가 tmpName 정리
		} else {
			cleanupTmp = false // rename 성공 → tmpName은 finalPath로 이동
		}
	}

	// 5) 영상 길이
	durationSeconds := durationHint
	if durationSeconds == 0 {
		if d, err := ProbeDurationSeconds(finalPath); err == nil && d > 0 {
			durationSeconds = d
		}
	}

	// 6) 썸네일 (영상 길이 기반 동적 시점, 최대 3초)
	at := 1
	if durationSeconds > 2 {
		if durationSeconds-1 < 3 {
			at = int(durationSeconds - 1)
		} else {
			at = 3
		}
	}
	thumbName := sumHex + ".jpg"
	thumbPath := filepath.Join(VideoThumbDirAbs, thumbName)
	if err := GenerateThumbnail(finalPath, thumbPath, at, 320); err != nil {
		// 썸네일 실패는 치명적이지 않음
		thumbPath = ""
	}

	return &VideoUploadResult{
		FinalName:       finalName,
		FinalPath:       finalPath,
		ThumbnailPath:   thumbPath,
		DurationSeconds: durationSeconds,
		IsPublic:        opts.IsPublic,
	}, nil
}
