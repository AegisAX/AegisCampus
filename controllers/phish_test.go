package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/AegisAX/AegisCampus/config"
	"github.com/AegisAX/AegisCampus/models"
)

func getFirstCampaign(t *testing.T) models.Campaign {
	campaigns, err := models.GetCampaigns(1)
	if err != nil {
		t.Fatalf("error getting first campaign from database: %v", err)
	}
	return campaigns[0]
}

func getFirstEmailRequest(t *testing.T) models.EmailRequest {
	campaign := getFirstCampaign(t)
	req := models.EmailRequest{
		TemplateId:    campaign.TemplateId,
		Template:      campaign.Template,
		PageId:        campaign.PageId,
		Page:          campaign.Page,
		URL:           "http://localhost.localdomain",
		UserId:        1,
		BaseRecipient: campaign.Results[0].BaseRecipient,
		SMTP:          campaign.SMTP,
		FromAddress:   campaign.SMTP.FromAddress,
	}
	err := models.PostEmailRequest(&req)
	if err != nil {
		t.Fatalf("error creating email request: %v", err)
	}
	return req
}

func openEmail(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
	got, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body from /track endpoint: %v", err)
	}
	expected, err := ioutil.ReadFile("static/images/pixel.png")
	if err != nil {
		t.Fatalf("error reading local transparent pixel: %v", err)
	}
	if !bytes.Equal(got, expected) {
		t.Fatalf("unexpected tracking pixel data received. expected %#v got %#v", expected, got)
	}
}

func openEmail404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/track?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expected, got)
	}
}

func reportedEmail(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/report?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /report endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNoContent
	if got != expected {
		t.Fatalf("invalid status code received for /report endpoint. expected %d got %d", expected, got)
	}
}

func reportEmail404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/report?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting /report endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /report endpoint. expected %d got %d", expected, got)
	}
}

func clickLink(t *testing.T, ctx *testContext, rid string, expectedHTML string) {
	resp, err := http.Get(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading payload from / endpoint response: %v", err)
	}
	if !bytes.Equal(got, []byte(expectedHTML)) {
		t.Fatalf("invalid response received from / endpoint. expected %s got %s", got, expectedHTML)
	}
}

func clickLink404(t *testing.T, ctx *testContext, rid string) {
	resp, err := http.Get(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
}

func transparencyRequest(t *testing.T, ctx *testContext, r models.Result, rid, path string) {
	resp, err := http.Get(fmt.Sprintf("%s%s?%s=%s", ctx.phishServer.URL, path, models.RecipientParameter, rid))
	if err != nil {
		t.Fatalf("error requesting %s endpoint: %v", path, err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
	tr := &TransparencyResponse{}
	err = json.NewDecoder(resp.Body).Decode(tr)
	if err != nil {
		t.Fatalf("error unmarshaling transparency request: %v", err)
	}
	expectedResponse := &TransparencyResponse{
		ContactAddress: ctx.config.ContactAddress,
		SendDate:       r.SendDate,
		Server:         config.ServerName,
	}
	if !reflect.DeepEqual(tr, expectedResponse) {
		t.Fatalf("unexpected transparency response received. expected %v got %v", expectedResponse, tr)
	}
}

func TestOpenedPhishingEmail(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}
	if lastEvent.Message != models.EventOpened {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventOpened)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestReportedPhishingEmail(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	reportedEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]

	if result.Reported != true {
		t.Fatalf("unexpected result report status received. expected %v got %v", true, result.Reported)
	}
	if lastEvent.Message != models.EventReported {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventReported)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestClickedPhishingLinkAfterOpen(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RId)
	clickLink(t, ctx, result.RId, campaign.Page.HTML)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	lastEvent := campaign.Events[len(campaign.Events)-1]
	if result.Status != models.EventClicked {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventClicked, result.Status)
	}
	if lastEvent.Message != models.EventClicked {
		t.Fatalf("unexpected event status received. expected %s got %s", lastEvent.Message, models.EventClicked)
	}
	if result.ModifiedDate != lastEvent.Time {
		t.Fatalf("unexpected result modified date received. expected %s got %s", lastEvent.Time, result.ModifiedDate)
	}
}

func TestNoRecipientID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/track", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting /track endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusNotFound
	if got != expected {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expected, got)
	}

	// Sentinel 동작: rid 없이 GET / 접근 시 신고 폼(/report-form) 으로 302 리다이렉트.
	// (controllers/phish.go 의 루트 라우터 참조)
	// http.Get 은 기본적으로 리다이렉트를 따라가므로, 따라가지 않는 클라이언트로 직접 검증한다.
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err = client.Get(ctx.phishServer.URL + "/")
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	got = resp.StatusCode
	expected = http.StatusFound
	if got != expected {
		t.Fatalf("invalid status code received for / endpoint. expected %d got %d", expected, got)
	}
	if loc := resp.Header.Get("Location"); loc != "/report-form" {
		t.Fatalf("invalid Location header for / endpoint. expected %q got %q", "/report-form", loc)
	}
}

func TestInvalidRecipientID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	rid := "XXXXXXXXXX"
	openEmail404(t, ctx, rid)
	clickLink404(t, ctx, rid)
	reportEmail404(t, ctx, rid)
}

func TestCompletedCampaignClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	if result.Status != models.StatusSending {
		t.Fatalf("unexpected result status received. expected %s got %s", models.StatusSending, result.Status)
	}

	openEmail(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}

	models.CompleteCampaign(campaign.Id, 1)
	openEmail404(t, ctx, result.RId)
	clickLink404(t, ctx, result.RId)

	campaign = getFirstCampaign(t)
	result = campaign.Results[0]
	if result.Status != models.EventOpened {
		t.Fatalf("unexpected result status received. expected %s got %s", models.EventOpened, result.Status)
	}
}

func TestRobotsHandler(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.Get(fmt.Sprintf("%s/robots.txt", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting /robots.txt endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expectedStatus := http.StatusOK
	if got != expectedStatus {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expectedStatus, got)
	}
	expected := []byte("User-agent: *\nDisallow: /\n")
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body from /robots.txt endpoint: %v", err)
	}
	if !bytes.Equal(body, expected) {
		t.Fatalf("invalid robots.txt response received. expected %s got %s", expected, body)
	}
}

func TestInvalidPreviewID(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	bogusRId := fmt.Sprintf("%sbogus", models.PreviewPrefix)
	openEmail404(t, ctx, bogusRId)
	clickLink404(t, ctx, bogusRId)
	reportEmail404(t, ctx, bogusRId)
}

func TestPreviewTrack(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	req := getFirstEmailRequest(t)
	openEmail(t, ctx, req.RId)
}

func TestPreviewClick(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	req := getFirstEmailRequest(t)
	clickLink(t, ctx, req.RId, req.Page.HTML)
}

func TestInvalidTransparencyRequest(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	bogusRId := fmt.Sprintf("bogus%s", TransparencySuffix)
	openEmail404(t, ctx, bogusRId)
	clickLink404(t, ctx, bogusRId)
	reportEmail404(t, ctx, bogusRId)
}

func TestTransparencyRequest(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	campaign := getFirstCampaign(t)
	result := campaign.Results[0]
	rid := fmt.Sprintf("%s%s", result.RId, TransparencySuffix)
	transparencyRequest(t, ctx, result, rid, "/")
	transparencyRequest(t, ctx, result, rid, "/track")
	transparencyRequest(t, ctx, result, rid, "/report")

	// And check with the URL encoded version of a +
	rid = fmt.Sprintf("%s%s", result.RId, "%2b")
	transparencyRequest(t, ctx, result, rid, "/")
	transparencyRequest(t, ctx, result, rid, "/track")
	transparencyRequest(t, ctx, result, rid, "/report")
}

func TestRedirectTemplating(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	p := models.Page{
		Name:        "Redirect Page",
		HTML:        "<html>Test</html>",
		UserId:      1,
		RedirectURL: "http://example.com/{{.RId}}",
	}
	err := models.PostPage(&p)
	if err != nil {
		t.Fatalf("error posting new page: %v", err)
	}
	smtp, _ := models.GetSMTP(1, 1)
	template, _ := models.GetTemplate(1, 1)
	group, _ := models.GetGroup(1, 1)

	campaign := models.Campaign{Name: "Redirect campaign"}
	campaign.UserId = 1
	campaign.Template = template
	campaign.Page = p
	campaign.SMTP = smtp
	campaign.Groups = []models.Group{group}
	err = models.PostCampaign(&campaign, campaign.UserId)
	if err != nil {
		t.Fatalf("error creating campaign: %v", err)
	}

	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	result := campaign.Results[0]
	resp, err := client.PostForm(fmt.Sprintf("%s/?%s=%s", ctx.phishServer.URL, models.RecipientParameter, result.RId), url.Values{"username": {"test"}, "password": {"test"}})
	if err != nil {
		t.Fatalf("error requesting / endpoint: %v", err)
	}
	defer resp.Body.Close()
	got := resp.StatusCode
	expectedStatus := http.StatusFound
	if got != expectedStatus {
		t.Fatalf("invalid status code received for /track endpoint. expected %d got %d", expectedStatus, got)
	}
	expectedURL := fmt.Sprintf("http://example.com/%s", result.RId)
	gotURL, err := resp.Location()
	if err != nil {
		t.Fatalf("error getting Location header from response: %v", err)
	}
	if gotURL.String() != expectedURL {
		t.Fatalf("invalid redirect received. expected %s got %s", expectedURL, gotURL)
	}
}

// TestTrackVideoServerAuthoritative is the rc1 F1 regression guard. POST
// /track/video must NOT trust the client-supplied "completed" boolean (the
// back door that bypassed the server-authoritative TrainingCompleteHandler),
// and must cap seconds_watched to the server-known duration. A legitimate
// watch (>=90% of videos.duration_seconds) must still complete so the
// reload / keep-completed flow (#10/#11) is not regressed.
func TestTrackVideoServerAuthoritative(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	// Server-authoritative duration = 100s (as ffprobe would set at upload).
	v := &models.Video{
		UserId:          1,
		Name:            "F1 Video",
		FileName:        "f1.mp4",
		FilePath:        "f1.mp4",
		DurationSeconds: 100,
	}
	if err := models.CreateVideo(v); err != nil {
		t.Fatalf("CreateVideo: %v", err)
	}

	smtp, _ := models.GetSMTP(1, 1)
	template, _ := models.GetTemplate(1, 1)
	page, _ := models.GetPage(1, 1)
	// F1 시나리오 = "LandingPage 임베드 영상 시청". #59 의 videoActionAllowed LP
	// 분기를 통과하도록 영상을 캠페인 LP 에 연결한다. 요청 시점 GetCampaign 이
	// page 를 PageId 로 DB 재로드하므로 video_id 를 영속화해야 한다.
	page.VideoId = &v.Id
	if err := models.PutPage(&page); err != nil {
		t.Fatalf("PutPage(link video to LP): %v", err)
	}
	group, _ := models.GetGroup(1, 1)
	campaign := models.Campaign{Name: "F1 campaign"}
	campaign.UserId = 1
	campaign.Template = template
	campaign.Page = page
	campaign.SMTP = smtp
	campaign.Groups = []models.Group{group}
	if err := models.PostCampaign(&campaign, campaign.UserId); err != nil {
		t.Fatalf("PostCampaign: %v", err)
	}
	rid := campaign.Results[0].RId

	// 실제 LP 영상 시청은 링크 클릭(Clicked) 이후에 일어난다. 결과를 Clicked 로
	// 진전시켜 시나리오를 현실과 일치시킨다(LP 분기는 status 무관하나 의미 명확화).
	clicked := campaign.Results[0]
	if err := clicked.HandleClickedLink(models.EventDetails{}); err != nil {
		t.Fatalf("HandleClickedLink: %v", err)
	}

	post := func(path string, body interface{}) int {
		b, _ := json.Marshal(body)
		resp, err := http.Post(ctx.phishServer.URL+path, "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	// Forgery: client claims completed:true with ~0 watch time.
	post("/track/video", map[string]interface{}{
		"rid": rid, "video_id": v.Id, "event": "ended",
		"seconds_watched": 0, "duration": 1, "completed": true,
	})
	vp, err := models.GetVideoProgress(1, campaign.Results[0].Id, v.Id)
	if err != nil {
		t.Fatalf("GetVideoProgress: %v", err)
	}
	if vp != nil && vp.Completed {
		t.Fatalf("forged completed:true was honored (vp.Completed=true)")
	}
	if code := post("/api/training/complete", map[string]interface{}{
		"rid": rid, "video_id": v.Id, "duration": 1, "watched": 0, "percent": 100,
	}); code == http.StatusOK {
		t.Fatalf("training complete accepted a forged record (200)")
	}

	// Legit watch: 95/100s = 95% >= 90% (server-computed).
	if code := post("/track/video", map[string]interface{}{
		"rid": rid, "video_id": v.Id, "event": "progress",
		"seconds_watched": 95, "duration": 100,
	}); code != http.StatusNoContent {
		t.Fatalf("legit /track/video expected 204, got %d", code)
	}
	vp, err = models.GetVideoProgress(1, campaign.Results[0].Id, v.Id)
	if err != nil || vp == nil {
		t.Fatalf("GetVideoProgress after legit watch: vp=%v err=%v", vp, err)
	}
	if !vp.Completed {
		t.Fatalf("legit 95%% watch did not complete server-side (regression for #10/#11)")
	}
	if code := post("/api/training/complete", map[string]interface{}{
		"rid": rid, "video_id": v.Id, "duration": 100, "watched": 95, "percent": 95,
	}); code != http.StatusOK {
		t.Fatalf("legit training complete expected 200, got %d", code)
	}

	// Cap: client over-reports seconds_watched beyond real duration.
	post("/track/video", map[string]interface{}{
		"rid": rid, "video_id": v.Id, "event": "progress",
		"seconds_watched": 999999, "duration": 100,
	})
	vp, _ = models.GetVideoProgress(1, campaign.Results[0].Id, v.Id)
	if vp != nil && vp.SecondsWatched > v.DurationSeconds {
		t.Fatalf("seconds_watched not capped to server duration (got %d > %d)",
			vp.SecondsWatched, v.DurationSeconds)
	}
}
