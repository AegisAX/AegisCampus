package util

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

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

// FfmpegBin은 ffmpeg 바이너리 경로입니다. 환경변수 GOPHISH_FFMPEG로 오버라이드 가능합니다.
var FfmpegBin = videoEnvDefault("GOPHISH_FFMPEG", "ffmpeg")

// FfprobeBin은 ffprobe 바이너리 경로입니다. 환경변수 GOPHISH_FFPROBE로 오버라이드 가능합니다.
var FfprobeBin = videoEnvDefault("GOPHISH_FFPROBE", "ffprobe")

func videoEnvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
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