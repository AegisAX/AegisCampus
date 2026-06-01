package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	gctx "github.com/AegisAX/AegisCampus/context"
	"github.com/AegisAX/AegisCampus/models"
	"github.com/gorilla/mux"
)

// getVideoByID 는 HandleVideoByID 의 GET 분기를 user_id 컨텍스트와 함께 직접
// 호출하는 테스트 헬퍼다. RequireLogin/RequireAPIKey 미들웨어가 평소 주입하는
// user_id 를 테스트에서 동일하게 세팅한다.
func getVideoByID(t *testing.T, ctx *testContext, videoID, asUserID int64) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/videos/%d", videoID), nil)
	req = mux.SetURLVars(req, map[string]string{"id": strconv.FormatInt(videoID, 10)})
	req = gctx.Set(req, "user_id", asUserID)
	resp := httptest.NewRecorder()
	ctx.apiServer.HandleVideoByID(resp, req)
	return resp.Code
}

// TestVideoGetCrossTenant 은 #61 회귀 가드다. 단건 GET /api/videos/{id} 가
// 목록(GetVideosForUser)·StreamVideo 와 동일한 own-or-public 불변식을 지키는지
// 검증한다. 비소유·비공개 영상은 존재 노출 방지를 위해 404 여야 한다.
func TestVideoGetCrossTenant(t *testing.T) {
	ctx := setupTest(t)

	// 다른 사용자(user_id=2) 소유의 비공개 영상
	priv := &models.Video{UserId: 2, Name: "Other Private", FileName: "o.mp4", FilePath: "o.mp4", IsPublic: false}
	if err := models.CreateVideo(priv); err != nil {
		t.Fatalf("CreateVideo(priv): %v", err)
	}
	// 다른 사용자(user_id=2) 소유의 공개 영상
	pub := &models.Video{UserId: 2, Name: "Other Public", FileName: "p.mp4", FilePath: "p.mp4", IsPublic: true}
	if err := models.CreateVideo(pub); err != nil {
		t.Fatalf("CreateVideo(pub): %v", err)
	}

	// admin(user_id=1) 이 남의 비공개 영상 조회 → 404 (IDOR 차단)
	if code := getVideoByID(t, ctx, priv.Id, 1); code != http.StatusNotFound {
		t.Fatalf("cross-tenant private video GET expected 404, got %d", code)
	}
	// 소유자(user_id=2) 본인 조회 → 200 (대조군)
	if code := getVideoByID(t, ctx, priv.Id, 2); code != http.StatusOK {
		t.Fatalf("owner private video GET expected 200, got %d", code)
	}
	// 비소유자라도 is_public 영상은 조회 가능 → 200 (목록 불변식과 일치)
	if code := getVideoByID(t, ctx, pub.Id, 1); code != http.StatusOK {
		t.Fatalf("public video GET expected 200, got %d", code)
	}
}
