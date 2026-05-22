package controllers

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AegisAX/Sentinel/config"
	ctx "github.com/AegisAX/Sentinel/context"
	"github.com/AegisAX/Sentinel/controllers/api"
	log "github.com/AegisAX/Sentinel/logger"
	"github.com/AegisAX/Sentinel/middleware"
	"github.com/AegisAX/Sentinel/models"
	"github.com/AegisAX/Sentinel/util"
	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// ErrInvalidRequest is thrown when a request with an invalid structure is
// received
var ErrInvalidRequest = errors.New("Invalid request")

// ErrCampaignComplete is thrown when an event is received for a campaign that
// has already been marked as complete.
var ErrCampaignComplete = errors.New("Event received on completed campaign")

// TransparencyResponse is the JSON response provided when a third-party
// makes a request to the transparency handler.
type TransparencyResponse struct {
	Server         string    `json:"server"`
	ContactAddress string    `json:"contact_address"`
	SendDate       time.Time `json:"send_date"`
}

// TransparencySuffix (when appended to a valid result ID), will cause Sentinel
// to return a transparency response.
const TransparencySuffix = "+"

// PhishingServerOption is a functional option that is used to configure the
// the phishing server
type PhishingServerOption func(*PhishingServer)

// PhishingServer is an HTTP server that implements the campaign event
// handlers, such as email open tracking, click tracking, and more.
type PhishingServer struct {
	server         *http.Server
	config         config.PhishServer
	contactAddress string
}

type PhishServer = PhishingServer

// miniSwalScript: 해킹 메일 신고 페이지용 내장 알림 (외부 의존성 없음)
// escHtml 헬퍼 추가, title/text/button에 적용
const miniSwalScript = `<script>(function(){
var _esc=function(s){var d=document.createElement("div");d.textContent=String(s||"");return d.innerHTML;};
var _sf=function(o){return new Promise(function(res){
var ov=document.createElement("div");
ov.style.cssText="position:fixed;inset:0;background:rgba(0,0,0,.45);display:flex;align-items:center;justify-content:center;z-index:99999";
var ic={"success":"✅","error":"❌","warning":"⚠️","info":"ℹ️"}[o.icon||""]||"";
ov.innerHTML="<div style=\"background:#fff;border-radius:12px;padding:32px 24px;max-width:360px;width:90%;text-align:center;box-shadow:0 8px 32px rgba(0,0,0,.18)\">"
+"<div style=\"font-size:48px;margin-bottom:12px\">"+ic+"</div>"
+(o.title?"<h2 style=\"margin:0 0 10px;font-size:22px;color:#2c3e50\">"+_esc(o.title)+"</h2>":"")
+(o.text?"<p style=\"margin:0 0 20px;color:#555;font-size:15px\">"+_esc(o.text)+"</p>":"")
+"<button style=\"background:#3085d6;color:#fff;border:0;border-radius:8px;padding:10px 28px;font-size:16px;font-weight:700;cursor:pointer\">"+_esc(o.confirmButtonText||"OK")+"</button>"
+"</div>";
ov.querySelector("button").onclick=function(){document.body.removeChild(ov);res({value:true});};
document.body.appendChild(ov);
});};
window.Swal={fire:_sf};
})();</script>`

// NewPhishingServer returns a new instance of the phishing server with
// provided options applied.
func NewPhishingServer(config config.PhishServer, options ...PhishingServerOption) *PhishingServer {
	defaultServer := &http.Server{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Addr:         config.ListenURL,
	}
	ps := &PhishingServer{
		server: defaultServer,
		config: config,
	}
	for _, opt := range options {
		opt(ps)
	}
	ps.registerRoutes()
	return ps
}

// WithContactAddress sets the contact address used by the transparency
// handlers
func WithContactAddress(addr string) PhishingServerOption {
	return func(ps *PhishingServer) {
		ps.contactAddress = addr
	}
}

// Start launches the phishing server, listening on the configured address.
func (ps *PhishingServer) Start() {
	if ps.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		ps.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(ps.config.CertPath, ps.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting phishing server at https://%s", ps.config.ListenURL)
		log.Fatal(ps.server.ListenAndServeTLS(ps.config.CertPath, ps.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting phishing server at http://%s", ps.config.ListenURL)
	log.Fatal(ps.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (ps *PhishingServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return ps.server.Shutdown(ctx)
}

// CreatePhishingRouter creates the router that handles phishing connections.
func (ps *PhishingServer) registerRoutes() {
	router := mux.NewRouter()
	fileServer := http.FileServer(middleware.NoIndexDir("./static/endpoint/"))
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fileServer))
	router.HandleFunc("/track", ps.TrackHandler)
	router.HandleFunc("/robots.txt", ps.RobotsHandler)
	router.HandleFunc("/{path:.*}/track", ps.TrackHandler)
	router.HandleFunc("/{path:.*}/report", ps.ReportHandler)
	router.HandleFunc("/report", ps.ReportHandler)

	// 첨부파일 열람 경로 추가
	router.HandleFunc("/fileopen", ps.FileOpenHandler).Methods("GET")

	// 신고 폼(메모 입력) 경로 추가
	router.HandleFunc("/report-form", ps.ReportFormGet).Methods("GET")
	router.HandleFunc("/report-form", ps.ReportFormPost).Methods("POST")

	// 동영상 관련 경로 추가
	router.HandleFunc("/media/{id:[0-9]+}", ps.Media).Methods("GET")
	router.HandleFunc("/track/video", ps.TrackVideo).Methods("POST")
	router.HandleFunc("/track/video/progress", ps.GetVideoProgress).Methods("GET")
	router.HandleFunc("/api/training/complete", TrainingCompleteHandler).Methods("POST")

	// Redirect Page 경로 추가
	router.HandleFunc("/rp/{id:[0-9]+}", ps.RedirectPageHandler).Methods("GET")

	// 루트("/") 처리
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rid := r.URL.Query().Get(models.RecipientParameter)
		if rid != "" {
			// rid 파라미터가 있으면 원래 피싱 랜딩 페이지 핸들러로 넘김
			ps.PhishHandler(w, r)
			return
		}
		// rid가 없으면 신고 폼으로 리다이렉트
		http.Redirect(w, r, "/report-form", http.StatusFound)
	}).Methods("GET")

	router.HandleFunc("/{path:.*}", ps.PhishHandler)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	phishHandler := gzipWrapper(router)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	phishHandler = handlers.ProxyHeaders(phishHandler)

	// Setup logging
	phishHandler = handlers.CombinedLoggingHandler(log.Writer(), phishHandler)
	ps.server.Handler = phishHandler
}

// /fileopen FileOpenHandler 추가
func (ps *PhishingServer) FileOpenHandler(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	rid := strings.TrimSpace(r.Form.Get(models.RecipientParameter)) // "rid"
	rawURL := strings.TrimSpace(r.Form.Get("url"))

	// 공백으로 끝나면 transparency "+" 보정
	if strings.HasSuffix(rid, " ") {
		rid = strings.TrimRight(rid, " ") + TransparencySuffix
	}

	// 실제 id
	id := strings.TrimSuffix(rid, TransparencySuffix)

	target := ""
	if rawURL != "" && isSafeInternalPath(rawURL) {
		target = rawURL
	} else {
		target = "/static/warning.html?rid=" + url.QueryEscape(id)
	}

	// '유효한 rid' + '프리뷰가 아닐 때'만 이벤트/업데이트 시도
	if rid != "" && !strings.HasPrefix(id, models.PreviewPrefix) {
		if rs, err := models.GetResult(id); err == nil {
			// (#26) 완료된 캠페인은 이벤트 기록 차단 (redirect 은 그대로 진행)
			campaign, cerr := models.GetCampaign(rs.CampaignId, rs.UserId)
			if cerr == nil && campaign.Status == models.CampaignComplete {
				log.Infof("fileopen: campaign complete (rid=%s, campaign_id=%d) — event skipped", id, campaign.Id)
			} else {
				// EventDetails 구성(간단 버전)
				d := models.EventDetails{
					Payload: r.Form,
					Browser: map[string]string{
						"user-agent": r.Header.Get("User-Agent"),
						"address":    r.RemoteAddr, // 필요하면 SplitHostPort 적용
					},
				}
				if err := rs.HandleAttachmentExecuted(d); err != nil {
					log.Errorf("fileopen: handle attachment open failed (rid=%s): %v", id, err)
				}
			}
		} else {
			log.Infof("fileopen: no result for rid=%s; redirecting to %s", id, target)
		}
	}

	http.Redirect(w, r, target, http.StatusFound)
}

// 내부 경로만 허용하도록 필터 기능 강화
func isSafeInternalPath(u string) bool {
	// F3: 백슬래시는 일부 브라우저가 '/' 로 정규화하여 protocol-relative
	// (//evil.com) 우회 → 오픈 리다이렉트가 된다. 제어문자도 헤더 분리/
	// 우회에 악용될 수 있어 함께 차단한다. 정상 내부경로/파일명에는 포함되지 않음.
	if strings.ContainsAny(u, "\\\x00\r\n\t") {
		return false
	}
	lu := strings.ToLower(u)
	// 1) 외부 스킴 명시적 차단
	for _, scheme := range []string{"http://", "https://", "data:", "javascript:", "vbscript:", "file://"} {
		if strings.HasPrefix(lu, scheme) {
			return false
		}
	}
	// 2) 앞뒤 공백 제거
	u = strings.TrimSpace(u)
	// 3) 프로토콜 상대 URL 차단 — "/" 검사보다 반드시 먼저 실행
	if strings.HasPrefix(u, "//") {
		return false
	}
	// 4) 절대/상대 경로 허용, 단 ".." 컴포넌트 포함 시 차단
	if strings.HasPrefix(u, "/") || strings.HasPrefix(u, "./") {
		if strings.Contains(u, "..") {
			return false
		}
		return true
	}
	// 5) 단순 파일명 허용 (":// "나 "//" 미포함), ".." 제외
	if !strings.Contains(u, "://") && !strings.HasPrefix(u, "//") {
		if strings.Contains(u, "..") {
			return false
		}
		return true
	}
	return false
}

// GET /report-form?rid=...
func (ps *PhishingServer) ReportFormGet(w http.ResponseWriter, r *http.Request) {
	rid := r.URL.Query().Get(models.RecipientParameter) // "rid"
	// rid 없어도 폼은 열 수 있도록 허용 (POST에서 fallback 매칭 처리)
	// if rid == "" { ... }  <-- 차단하지 않음

	// 주의: miniSwalScript 안에 리터럴 '%' 가 포함돼 있어 fmt.Fprintf 의 포맷 문자열로 바로 넘길 수 없습니다 ("%; has unknown verb ;" 오류).
	// 그래서 정적 부분은 io.WriteString 으로, rid 가 들어가는 부분만 fmt.Fprintf 로 처리하도록 분리했습니다.
	io.WriteString(w, `<!doctype html>
<html lang="ko">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>해킹 메일 신고</title>
<style>
	:root{
		--header-top:#253445; --header-bot:#0f1a24;
		--bg:#f5f7fa; --panel:#ffffff; --border:#e1e5ea; --heading:#2c3e50;
		--text:#2b2f33; --muted:#6c7a89; --primary:#337ab7; --primary-dark:#286090;
		--control-bg:#fff; --control-border:#ccd4dd; --chip-bg:#f8fafc;
	}
	*{box-sizing:border-box} html,body{height:100%}
	body{
		margin:0;
		font:14px/1.45 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,"Apple SD Gothic Neo","Noto Sans KR",sans-serif;
		color:var(--text);
		background:
			linear-gradient(180deg, var(--header-top), var(--header-bot) 280px),
			var(--bg);
		background-repeat:no-repeat;
	}
	.container{max-width:900px;margin:40px auto;padding:0 20px}
	.panel{background:var(--panel);border:1px solid var(--border);border-radius:4px;box-shadow:0 1px 2px rgba(0,0,0,.03)}
	.panel-heading{padding:14px 18px;border-bottom:1px solid var(--border);text-align:center}
	.panel-heading h1{margin:0;font-size:32px;color:var(--heading);font-weight:800}
	.panel-body{padding:18px}
	.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}
	.row-2{grid-column:1 / -1}
	.form-group{margin-bottom:14px}
	label{display:block;margin-bottom:6px;color:#44525f;font-weight:600;font-size:13px}
	.form-control{
		width:100%;display:block;padding:8px 10px;border:1px solid var(--control-border);
    border-radius:4px;background:var(--control-bg);color:var(--text);outline:none;transition:border .15s ease
	}
	.form-control:focus{border-color:#99b5d1}
	.form-control::placeholder {color:#aeb6bf;opacity:1;}
	.help{font-size:12px;color:var(--muted);margin-top:4px}
	.chips{display:flex;flex-wrap:wrap;gap:10px}
	.chip{display:inline-flex;align-items:center;gap:6px;padding:7px 10px;border:1px solid var(--control-border);border-radius:999px;background:var(--chip-bg)}
	.panel-footer{padding:14px 18px;border-top:1px solid var(--border);display:flex;gap:10px;justify-content:flex-end;background:#fafbfd}
	.btn{display:inline-block;padding:8px 14px;border-radius:4px;text-decoration:none;cursor:pointer;border:1px solid transparent;font-weight:600}
	.btn-default{background:#fff;border-color:var(--control-border);color:var(--heading)}
	.btn-default:hover{background:#f6f8fb}
	.btn-primary{background:var(--primary);border-color:var(--primary-dark);color:#fff}
	.btn-primary:hover{background:var(--primary-dark)}
	.alert{display:none;margin:0 18px 14px;border:1px solid #f1c4c0;background:#fdecea;color:#6b1b12;border-radius:4px;padding:10px 12px}
	.alert.show{display:block}
	@media (max-width:720px){ .form-grid{grid-template-columns:1fr} }
</style>
`)
	io.WriteString(w, miniSwalScript)
	fmt.Fprintf(w, `
</head>
<body>
<div class="container">
    <div class="panel">
		<div class="panel-heading"><h1>해킹 메일 신고</h1></div>
		<div id="formAlert" class="alert" role="alert" aria-live="polite"></div>
		<form id="reportForm" method="POST" action="/report-form" novalidate>
			<div class="panel-body">
				<!-- RID: 있을 수도/없을 수도 있음 -->
				<input type="hidden" name="rid" value="%s"/>
				<!-- 숨김: 메일 열람 일시(자동 설정) -->
				<input type="hidden" name="viewed_at" id="viewed_at"/>
				<div class="form-grid">
					<div class="form-group">
						<label for="reporter_name">신고자</label>
						<input class="form-control" id="reporter_name" type="text" name="reporter_name" placeholder="홍길동" required minlength="2" maxlength="50">
					</div>
					<div class="form-group">
						<label for="reporter_email">신고자 메일</label>
						<input class="form-control" id="reporter_email" type="email" name="reporter_email" placeholder="user@example.com" required maxlength="120">
					</div>
					<div class="form-group">
						<label for="mail_subject">해킹메일 제목</label>
						<input class="form-control" id="mail_subject" type="text" name="mail_subject" placeholder="[인사팀] 급여 정정 안내" required minlength="2" maxlength="200">
					</div>
					<div class="form-group">
						<label for="mail_from">해킹메일 주소</label>
						<input class="form-control" id="mail_from" type="email" name="mail_from" placeholder="hr@example.com" required maxlength="120">
					</div>
				</div>
			</div>
			<div class="panel-footer">
				<button type="reset" class="btn btn-default">초기화</button>
				<button type="submit" class="btn btn-primary">해킹 메일 신고</button>
			</div>
		</form>
	</div>
</div>

<script>
// 숨김 viewed_at 자동 값
(function(){
	var el = document.getElementById('viewed_at');
	if (el && !el.value) {
		const d = new Date();
		d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
		el.value = d.toISOString();
	}
})();

// 유효성 + 체크 최소 1개
(function(){
	const form = document.getElementById('reportForm');
	const alertBox = document.getElementById('formAlert');
	// 수정 — fetch로 AJAX 제출 후 팝업 처리
	form.addEventListener('submit', function(e){
		e.preventDefault();
		alertBox.classList.remove('show');
		alertBox.textContent = '';

		if (!form.checkValidity()) {
			alertBox.textContent = '필수 항목을 올바르게 입력해 주세요.';
			alertBox.classList.add('show');
			form.reportValidity();
			return;
		}

		var submitBtn = form.querySelector('button[type="submit"]');
		submitBtn.disabled = true;
		submitBtn.textContent = '처리 중...';

		var formData = new URLSearchParams(new FormData(form));
		fetch('/report-form', {
			method: 'POST',
			headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
			body: formData
		})
		.then(function(res) {
			if (!res.ok) throw new Error('서버 오류');
			return res.json();
		})
		.then(function(data) {
			if (data.success) {
				Swal.fire({title:'신고 완료',text:'해킹 메일 신고가 완료되었습니다. 감사합니다!',icon:'success',confirmButtonText:'확인'})
				.then(function(){
					try { window.close(); } catch(e) {}
					setTimeout(function() {
						document.body.innerHTML = '<p style="text-align:center;margin-top:50px;font-size:18px;">창을 닫아주세요.</p>';
					}, 300);
				});
			}
		})
		.catch(function() {
			alertBox.textContent = '신고 처리 중 오류가 발생했습니다. 다시 시도해 주세요.';
			alertBox.classList.add('show');
			submitBtn.disabled = false;
			submitBtn.textContent = '해킹 메일 신고';
		});
	}, false);
})();
</script>
</body>
</html>`, html.EscapeString(rid))
}

// POST /report-form
func (ps *PhishingServer) ReportFormPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	// 입력값
	get := func(k string) string { return strings.TrimSpace(r.Form.Get(k)) }
	rid := get(models.RecipientParameter)
	viewedAt := get("viewed_at")
	reporterName := get("reporter_name")
	reporterEmail := get("reporter_email") // 수신자 이메일
	mailFrom := get("mail_from")           // 발신자 이메일
	mailSubject := get("mail_subject")
	yn := func(k string) string {
		if strings.ToLower(get(k)) == "yes" {
			return "Yes"
		}
		return "No"
	}
	opened := yn("opened")
	clicked := yn("clicked")
	submitted := yn("submitted")
	downloaded := yn("downloaded")
	executed := yn("executed")

	// --- 이메일 형식 서버측 검증 개선 ---
	if len(reporterName) < 2 || len(mailSubject) < 2 {
		http.Error(w, "invalid input", http.StatusBadRequest)
		return
	}
	if _, err := mail.ParseAddress(reporterEmail); err != nil {
		http.Error(w, "invalid reporter email", http.StatusBadRequest)
		return
	}
	if _, err := mail.ParseAddress(mailFrom); err != nil {
		http.Error(w, "invalid from email", http.StatusBadRequest)
		return
	}
	// -------------------

	// note: 모든 매칭 결과에 동일한 신고 내용 기록
	noteText := fmt.Sprintf(
		`[신고 일시: %s] [신고자: %s (%s)] [발신자: %s] [메일 제목: %s] [메일 열람: %s] [링크 클릭: %s] [정보 입력: %s] [파일 다운로드: %s] [파일 실행: %s]`,
		viewedAt, reporterName, reporterEmail, mailFrom, mailSubject, opened, clicked, submitted, downloaded, executed,
	)

	d := models.EventDetails{
		Payload: r.Form,
		Browser: map[string]string{},
	}

	// 1) rid 우선 시도
	var targets []*models.Result
	if rid != "" {
		id := strings.TrimSuffix(rid, TransparencySuffix)
		if rs, err := models.GetResult(id); err == nil {
			targets = append(targets, &rs)
		}
	}

	// 2) rid 실패 시 → 이메일+제목으로 매칭 (다중 결과 처리)
	if len(targets) == 0 {
		matched, err := models.FindResultByEmailAndRenderedSubject(reporterEmail, mailSubject)
		if err != nil || len(matched) == 0 {
			log.Infof("report-form: no match by email+subject (email=%s, subject=%s) → thank-you", reporterEmail, mailSubject)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))
			return
		}
		targets = matched
	}

	// 모든 매칭 결과에 신고 처리
	for _, res := range targets {
		res.ReportNote = noteText
		if err := res.HandleEmailReport(d); err != nil {
			log.Errorf("report-form: HandleEmailReport failed (rid=%s): %v", res.RId, err)
			// 일부 실패해도 나머지는 계속 처리
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"success":true}`))
}

// TrackHandler tracks emails as they are opened, updating the status for the given Result
func (ps *PhishingServer) TrackHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		http.ServeFile(w, r, "static/images/pixel.png")
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailOpened(d)
	if err != nil {
		log.Error(err)
	}
	http.ServeFile(w, r, "static/images/pixel.png")
}

// ReportHandler tracks emails as they are reported, updating the status for the given Result
func (ps *PhishingServer) ReportHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	w.Header().Set("Access-Control-Allow-Origin", "*") // To allow Chrome extensions (or other pages) to report a campaign without violating CORS
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	// Check for a preview
	if _, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	err = rs.HandleEmailReport(d)
	if err != nil {
		log.Error(err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// PhishHandler handles incoming client connections and registers the associated actions performed
// (such as clicked link, etc.)
func (ps *PhishingServer) PhishHandler(w http.ResponseWriter, r *http.Request) {
	r, err := setupContext(r)
	if err != nil {
		// Log the error if it wasn't something we can safely ignore
		if err != ErrInvalidRequest && err != ErrCampaignComplete {
			log.Error(err)
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Server", config.ServerName) // Useful for checking if this is a Sentinel server (e.g. for campaign reporting plugins)
	var ptx models.PhishingTemplateContext
	// Check for a preview
	if preview, ok := ctx.Get(r, "result").(models.EmailRequest); ok {
		ptx, err = models.NewPhishingTemplateContext(&preview, preview.BaseRecipient, preview.RId)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
		p, err := models.GetPage(preview.PageId, preview.UserId)
		if err != nil {
			log.Error(err)
			http.NotFound(w, r)
			return
		}
		renderPhishResponse(w, r, ptx, p)
		return
	}
	rs := ctx.Get(r, "result").(models.Result)
	rid := ctx.Get(r, "rid").(string)
	c := ctx.Get(r, "campaign").(models.Campaign)
	d := ctx.Get(r, "details").(models.EventDetails)

	// Check for a transparency request
	if strings.HasSuffix(rid, TransparencySuffix) {
		ps.TransparencyHandler(w, r)
		return
	}

	p, err := models.GetPage(c.PageId, c.UserId)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}
	switch {
	case r.Method == "GET":
		err = rs.HandleClickedLink(d)
		if err != nil {
			log.Error(err)
		}
	case r.Method == "POST":
		err = rs.HandleFormSubmit(d)
		if err != nil {
			log.Error(err)
		}
	}
	ptx, err = models.NewPhishingTemplateContext(&c, rs.BaseRecipient, rs.RId)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
	}
	renderPhishResponse(w, r, ptx, p)
}

// renderPhishResponse handles rendering the correct response to the phishing
// connection. This usually involves writing out the page HTML or redirecting
// the user to the correct URL.
func renderPhishResponse(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext, p models.Page) {
	// If the request was a form submit and a redirect URL was specified, we
	// should send the user to that URL
	if r.Method == "POST" {
		if p.RedirectURL != "" {
			redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
			if err != nil {
				log.Error(err)
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}
	}
	// Otherwise, we just need to write out the templated HTML
	html, err := models.ExecuteTemplate(p.HTML, ptx)
	if err != nil {
		log.Error(err)
		http.NotFound(w, r)
		return
	}

	// RedirectURL이 설정된 경우 페이지 이탈/닫기 시 자동 리다이렉트 스크립트 주입
	// Landing Page HTML에 별도 코드 없이 서버에서 자동 처리
	if p.RedirectURL != "" {
		redirectURL, err := models.ExecuteTemplate(p.RedirectURL, ptx)
		if err == nil && redirectURL != "" {
			script := fmt.Sprintf(`<script>(function(){
				var _r=%q, _done=false;

				// 폼 제출 시 서버 POST 리다이렉트 처리 → 중복 방지
				document.addEventListener('submit',function(){_done=true;},true);

				// window.close() 가로채기 → 나중에 하기 버튼 등
				var _orig=window.close.bind(window);
				window.close=function(){
					if(!_done){_done=true;window.location.replace(_r);setTimeout(_orig,400);}
					else{_orig();}
				};

				// 뒤로가기 감지: 가짜 히스토리 항목 추가 후 popstate로 가로채기
				history.pushState({_sentinel:true},'');
				window.addEventListener('popstate',function(e){
					if(!_done){_done=true;window.location.replace(_r);}
					else{history.back();}
				});

				// beforeunload 폴백
				window.addEventListener('beforeunload',function(){
					if(!_done){_done=true;window.location.replace(_r);}
				});
				})();</script>`, redirectURL)
			if strings.Contains(html, "</body>") {
				html = strings.Replace(html, "</body>", script+"</body>", 1)
			} else {
				html += script
			}
		}
	}

	html = ensureMiniSwal(html)
	w.Write([]byte(html))
}

// ensureMiniSwal injects the self-contained Swal polyfill (miniSwalScript)
// into page HTML only when the page calls Swal.fire(...) but does not define
// window.Swal. Some Landing/Redirect pages were saved before the page builder
// JS embedded the polyfill, so their stored HTML invokes Swal without it being
// defined. The guard makes injection idempotent (정의가 이미 있으면 미주입,
// Swal 미사용 페이지에도 미주입).
func ensureMiniSwal(htmlStr string) string {
	if !strings.Contains(htmlStr, "Swal.fire") || strings.Contains(htmlStr, "window.Swal") {
		return htmlStr
	}
	switch {
	case strings.Contains(htmlStr, "</head>"):
		return strings.Replace(htmlStr, "</head>", miniSwalScript+"</head>", 1)
	case strings.Contains(htmlStr, "</body>"):
		return strings.Replace(htmlStr, "</body>", miniSwalScript+"</body>", 1)
	default:
		return miniSwalScript + htmlStr
	}
}

// RobotsHandler prevents search engines, etc. from indexing phishing materials
func (ps *PhishingServer) RobotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "User-agent: *\nDisallow: /")
}

// TransparencyHandler returns a TransparencyResponse for the provided result
// and campaign.
func (ps *PhishingServer) TransparencyHandler(w http.ResponseWriter, r *http.Request) {
	rs := ctx.Get(r, "result").(models.Result)
	tr := &TransparencyResponse{
		Server:         config.ServerName,
		SendDate:       rs.SendDate,
		ContactAddress: ps.contactAddress,
	}
	api.JSONResponse(w, tr, http.StatusOK)
}

// setupContext handles some of the administrative work around receiving a new
// request, such as checking the result ID, the campaign, etc.
func setupContext(r *http.Request) (*http.Request, error) {
	err := r.ParseForm()
	if err != nil {
		log.Error(err)
		return r, err
	}
	rid := r.Form.Get(models.RecipientParameter)
	if rid == "" {
		return r, ErrInvalidRequest
	}
	// Since we want to support the common case of adding a "+" to indicate a
	// transparency request, we need to take care to handle the case where the
	// request ends with a space, since a "+" is technically reserved for use
	// as a URL encoding of a space.
	if strings.HasSuffix(rid, " ") {
		// We'll trim off the space
		rid = strings.TrimRight(rid, " ")
		// Then we'll add the transparency suffix
		rid = fmt.Sprintf("%s%s", rid, TransparencySuffix)
	}
	// Finally, if this is a transparency request, we'll need to verify that
	// a valid rid has been provided, so we'll look up the result with a
	// trimmed parameter.
	id := strings.TrimSuffix(rid, TransparencySuffix)
	// Check to see if this is a preview or a real result
	if strings.HasPrefix(id, models.PreviewPrefix) {
		rs, err := models.GetEmailRequestByResultId(id)
		if err != nil {
			return r, err
		}
		r = ctx.Set(r, "result", rs)
		return r, nil
	}
	rs, err := models.GetResult(id)
	if err != nil {
		return r, err
	}
	c, err := models.GetCampaign(rs.CampaignId, rs.UserId)
	if err != nil {
		log.Error(err)
		return r, err
	}
	// Don't process events for completed campaigns
	if c.Status == models.CampaignComplete {
		return r, ErrCampaignComplete
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	// Handle post processing such as GeoIP
	err = rs.UpdateGeo(ip)
	if err != nil {
		log.Error(err)
	}
	d := models.EventDetails{
		Payload: r.Form,
		Browser: make(map[string]string),
	}
	d.Browser["address"] = ip
	d.Browser["user-agent"] = r.Header.Get("User-Agent")

	r = ctx.Set(r, "rid", rid)
	r = ctx.Set(r, "result", rs)
	r = ctx.Set(r, "campaign", c)
	r = ctx.Set(r, "details", d)
	return r, nil
}

// Media - 공개 스트리밍 엔드포인트 (Range 지원 via http.ServeContent)
// GET /media/{id}
func (ps *PhishingServer) Media(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, _ := strconv.ParseInt(idStr, 10, 64)

	// Phase 2 #7: rid 기반 캠페인-영상 연결 검증.
	// 외부 직접 접근 (ID 추측) 으로 영상이 노출되는 것을 차단한다.
	// 정상 캠페인 수신자는 RedirectPage/LandingPage 의 <source> 태그가
	// 렌더 시점에 ?rid={{.RId}} 토큰을 포함하므로 자연스럽게 통과한다.
	rid := r.URL.Query().Get(models.RecipientParameter)
	if rid == "" {
		log.Errorf("media: rid missing (id=%s, ip=%s)", idStr, r.RemoteAddr)
		http.NotFound(w, r)
		return
	}
	// F4: 다른 핸들러(FileOpen/TrainingComplete)와 동일하게 투명성 접미사 제거.
	rid = strings.TrimSuffix(rid, TransparencySuffix)
	result, err := models.GetResult(rid)
	if err != nil {
		log.Errorf("media: invalid rid (id=%s, rid=%s)", idStr, rid)
		http.NotFound(w, r)
		return
	}
	campaign, err := models.GetCampaign(result.CampaignId, result.UserId)
	if err != nil {
		log.Errorf("media: campaign lookup failed (id=%s, rid=%s, campaign_id=%d)", idStr, rid, result.CampaignId)
		http.NotFound(w, r)
		return
	}
	// (#21) 완료된 캠페인은 영상 송출 차단 — 410 Gone
	if campaign.Status == models.CampaignComplete {
		log.Infof("media: campaign complete (id=%s, rid=%s, campaign_id=%d)", idStr, rid, campaign.Id)
		http.Error(w, "Campaign complete", http.StatusGone)
		return
	}
	// Phase 2 #7: 영상이 result 소유자의 LandingPage 또는 RedirectPage 중
	// 어느 한 곳에라도 연결되어 있으면 통과시킨다.
	// RedirectPage 안에만 영상이 임베드된 시나리오 (campaign.Page.VideoId == nil)
	// 도 정상 처리되도록 함.
	linked, lerr := models.IsVideoLinkedToUser(id, result.UserId)
	if lerr != nil {
		log.Errorf("media: video link lookup failed (id=%s, rid=%s): %v", idStr, rid, lerr)
		http.NotFound(w, r)
		return
	}
	if !linked {
		log.Errorf("media: rid %s (campaign=%d, user=%d) does not authorize video %d", rid, campaign.Id, result.UserId, id)
		http.NotFound(w, r)
		return
	}

	v, err := models.GetVideo(id)
	if err != nil || v == nil || v.Id == 0 {
		log.Errorf("media: video not found (id=%s)", idStr)
		http.NotFound(w, r)
		return
	}
	path := v.FilePath
	// 상대경로면 절대경로로 변환 (util.VideoStorageDirAbs 기준)
	if !filepath.IsAbs(path) {
		path = filepath.Join(util.VideoStorageDirAbs, filepath.Base(path))
	}

	// path traversal 방어 (이제 base/target 모두 절대경로)
	if !util.IsUnderBaseDir(util.VideoStorageDirAbs, path) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		log.Errorf("media: cannot open %s: %v", path, err)
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		log.Errorf("media: stat failed: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", util.MimeTypeForFileName(v.FileName))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(path), fi.ModTime(), f)
}

// 트래킹 페이로드 구조체
type videoTrackPayload struct {
	RID            string `json:"rid"`
	VideoID        int64  `json:"video_id"`
	Event          string `json:"event"`           // play | progress | ended | unload
	SecondsWatched int64  `json:"seconds_watched"` // 변경
	Duration       int64  `json:"duration"`        // seconds (optional)
	Completed      bool   `json:"completed"`       // 추가
}

// TrackVideo - 랜딩 페이지에서 시청 이벤트(Beacon 등) 수신, DB에 누적/업데이트
// POST /track/video
func (ps *PhishingServer) TrackVideo(w http.ResponseWriter, r *http.Request) {
	var p videoTrackPayload
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err := dec.Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if p.RID == "" || p.VideoID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// RID -> Result 조회 (models.GetResultByRID 구현 필요)
	// 캠페인 완료 체크 추가
	res, err := models.GetResultByRID(p.RID)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if res == nil || res.Id == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// I-06: 완료된 캠페인은 비디오 트래킹 기록 중단
	if campaign, err := models.GetCampaign(res.CampaignId, res.UserId); err == nil {
		if campaign.Status == models.CampaignComplete {
			w.WriteHeader(http.StatusGone) // 410 Gone — 완료된 캠페인
			return
		}
	}

	// 기존 진행 기록 조회/생성
	vp, err := models.GetVideoProgress(res.UserId, res.Id, p.VideoID)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if vp == nil {
		vp = &models.VideoProgress{
			UserId:   res.UserId,
			ResultId: res.Id,
			VideoId:  p.VideoID,
		}
	}

	// F1: 서버 권위 갱신.
	// 클라가 보낸 completed 불리언은 신뢰하지 않는다(1필드 위조 차단).
	// 완료 판정은 서버 보유 videos.duration_seconds 기준 percent 로만 결정한다.
	// duration_seconds 가 0(ffprobe 실패) 인 경우에만 클라 duration 으로 폴백하되,
	// 이때도 클라 completed 는 무시하고 percent>=0.90 계산으로만 완료 처리한다.
	// (TrainingCompleteHandler 의 서버 권위 모델과 일관)
	var effDur int64
	if vd, gverr := models.GetVideo(p.VideoID); gverr == nil && vd != nil && vd.DurationSeconds > 0 {
		effDur = vd.DurationSeconds
	} else if p.Duration > 0 {
		effDur = p.Duration
	}

	sw := p.SecondsWatched
	if sw < 0 {
		sw = 0
	}
	if effDur > 0 && sw > effDur {
		sw = effDur // 서버 길이 상한으로 클램프 → percent 부풀리기 차단
	}
	// 단조 증가: 최댓값만 유지 (이어보기 / #10 새로고침)
	if sw > vp.SecondsWatched {
		vp.SecondsWatched = sw
	}

	if effDur > 0 {
		vp.Duration = effDur
		vp.Percent = float64(vp.SecondsWatched) / float64(effDur)
		if vp.Percent > 1 {
			vp.Percent = 1
		}
		// 완료 판정: 서버 길이 기준 90% 이상만. 한 번 true 면 유지.
		if vp.Percent >= 0.90 {
			vp.Completed = true
		}
	}

	if err := vp.Save(); err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type videoProgressResponse struct {
	SecondsWatched int64   `json:"seconds_watched"`
	Duration       int64   `json:"duration"`
	Percent        float64 `json:"percent"`
	Completed      bool    `json:"completed"`
}

func (ps *PhishingServer) GetVideoProgress(w http.ResponseWriter, r *http.Request) {
	rid := r.URL.Query().Get("rid")
	videoIDStr := r.URL.Query().Get("video_id")
	if rid == "" || videoIDStr == "" {
		http.Error(w, "missing rid or video_id", http.StatusBadRequest)
		return
	}
	videoID, err := strconv.ParseInt(videoIDStr, 10, 64)
	if err != nil || videoID <= 0 {
		http.Error(w, "invalid video_id", http.StatusBadRequest)
		return
	}

	// 결과 조회 (여기서 r_id 로 조회)
	// 캠페인 완료 체크 추가
	res, err := models.GetResultByRID(rid)
	if err != nil || res == nil || res.Id == 0 {
		http.Error(w, "result not found", http.StatusNotFound)
		return
	}

	// I-06: 완료된 캠페인은 진행률 조회 허용 (읽기는 허용, 쓰기만 차단)
	// GetVideoProgress는 읽기 전용이므로 완료 여부와 무관하게 조회 허용

	// 진행률 조회
	vp, err := models.GetVideoProgress(res.UserId, res.Id, videoID)
	if err != nil {
		log.Error(err)
		http.Error(w, "error", http.StatusInternalServerError)
		return
	}
	if vp == nil {
		api.JSONResponse(w, videoProgressResponse{}, http.StatusOK)
		return
	}
	api.JSONResponse(w, videoProgressResponse{
		SecondsWatched: vp.SecondsWatched,
		Duration:       vp.Duration,
		Percent:        vp.Percent,
		Completed:      vp.Completed,
	}, http.StatusOK)
}

// 1) video_id를 숫자/문자열 모두 수용하도록 변경
type trainingCompleteRequest struct {
	RID      string      `json:"rid"`
	VideoID  interface{} `json:"video_id"` // ← string 대신 interface{}
	Duration float64     `json:"duration"`
	Watched  float64     `json:"watched"`
	Percent  float64     `json:"percent"`
}

func TrainingCompleteHandler(w http.ResponseWriter, r *http.Request) {
	var req trainingCompleteRequest

	// JSON 파싱
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	rid := strings.TrimSpace(req.RID)
	if rid == "" {
		api.JSONResponse(w, models.Response{Success: false, Message: "missing rid"}, http.StatusBadRequest)
		return
	}

	// rid 투명성 접미사 제거 후 Result 조회
	id := strings.TrimSuffix(rid, TransparencySuffix)
	rs, err := models.GetResult(id)
	if err != nil {
		api.JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusNotFound)
		return
	}

	// 캠페인 완료 시 수강 완료 기록 차단 (TrackVideo / setupContext 와 동일 정책)
	if c, cerr := models.GetCampaign(rs.CampaignId, rs.UserId); cerr == nil {
		if c.Status == models.CampaignComplete {
			api.JSONResponse(w, models.Response{Success: false, Message: "campaign complete"}, http.StatusGone)
			return
		}
	}

	// video_id를 문자열로 정규화 후 정수 검증
	var vidStr string
	switch v := req.VideoID.(type) {
	case string:
		vidStr = v
	case float64: // JSON 숫자는 기본적으로 float64로 들어옵니다.
		// 정수처럼 보이면 정수로, 아니면 그대로
		if v == float64(int64(v)) {
			vidStr = strconv.FormatInt(int64(v), 10)
		} else {
			vidStr = strconv.FormatFloat(v, 'f', -1, 64)
		}
	case json.Number:
		vidStr = v.String()
	default:
		vidStr = fmt.Sprint(v)
	}
	vidID, verr := strconv.ParseInt(vidStr, 10, 64)
	if verr != nil || vidID <= 0 {
		api.JSONResponse(w, models.Response{Success: false, Message: "invalid video_id"}, http.StatusBadRequest)
		return
	}

	// 수강 완료 자격 — 서버 권위 기록만 신뢰.
	// 클라가 보낸 duration/watched/percent 는 자격 판단에 일절 사용하지 않는다.
	// /track/video 가 단조 누적한 video_progresses + videos.duration_seconds 로
	// 서버가 직접 판정한다. 정상 시청/#10 새로고침은 통과, 미시청 위조는 거부.
	vp, perr := models.GetVideoProgress(rs.UserId, rs.Id, vidID)
	if perr != nil {
		api.JSONResponse(w, models.Response{Success: false, Message: perr.Error()}, http.StatusInternalServerError)
		return
	}
	eligible := false
	if vp != nil {
		switch {
		case vp.Completed || vp.Percent >= 0.90:
			eligible = true
		case vp.SecondsWatched > 0:
			// 레이스 보강: ended 비콘(/track/video) 커밋 전에 완료 클릭이 먼저
			// 도착한 경우 대비. 클라값이 아닌 서버 보유 videos.duration_seconds 로 재계산.
			if vd, gverr := models.GetVideo(vidID); gverr == nil && vd != nil && vd.DurationSeconds > 0 {
				if float64(vp.SecondsWatched)/float64(vd.DurationSeconds) >= 0.90 {
					eligible = true
				}
			}
		}
	}
	if !eligible {
		api.JSONResponse(w, models.Response{Success: false, Message: "insufficient watch time"}, http.StatusBadRequest)
		return
	}

	// 원격 IP
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	// EventDetails (Browser=map[string]string, Payload=url.Values)
	payload := url.Values{}
	payload.Set("video_id", vidStr)
	if req.Duration > 0 {
		payload.Set("duration", strconv.FormatFloat(req.Duration, 'f', -1, 64))
	}
	if req.Watched > 0 {
		payload.Set("watched", strconv.FormatFloat(req.Watched, 'f', -1, 64))
	}
	if req.Percent > 0 {
		payload.Set("percent", strconv.FormatFloat(req.Percent, 'f', -1, 64))
	}
	payload.Set("address", ip)

	details := models.EventDetails{
		Browser: map[string]string{
			"user-agent": r.UserAgent(),
			"address":    ip,
		},
		Payload: payload,
	}

	if err := rs.HandleTrainingCompleted(details); err != nil {
		api.JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// B-04: VideoProgress.completed 동기화.
	// HandleTrainingCompleted 는 events 에 Trained 만 기록하므로 vp.completed 만 맞춘다.
	// SecondsWatched/Duration/Percent 는 /track/video 가 소유하는 서버 권위값이므로
	// 클라값으로 덮어쓰지 않는다. 자격 검증에서 vp 는 non-nil 이 보장된다.
	if vp != nil && !vp.Completed {
		vp.Completed = true
		if err := vp.Save(); err != nil {
			log.Warnf("TrainingComplete: VideoProgress 동기화 실패 (rid=%s, vid=%d): %v", rs.RId, vidID, err)
			// 동기화 실패는 non-fatal — Trained 이벤트는 이미 기록됨
		}
	}

	api.JSONResponse(w, models.Response{Success: true}, http.StatusOK)
}

// RedirectPageHandler serves a registered Redirect Page by its ID.
// GET /rp/{id}?rid={rid}
func (ps *PhishingServer) RedirectPageHandler(w http.ResponseWriter, r *http.Request) {
	idStr := mux.Vars(r)["id"]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		http.NotFound(w, r)
		return
	}

	rp, err := models.GetRedirectPageByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// rid가 있으면 PhishingTemplateContext 구성
	rid := r.URL.Query().Get("rid")
	if rid == "" {
		rid = r.URL.Query().Get("RId")
	}

	var pageHTML string
	if rid != "" {
		result, err := models.GetResult(rid)
		if err == nil {
			// I-07: rid와 RedirectPage 소유자가 동일한지 검증
			// 다른 사용자의 rid로 개인정보(이름/부서/이메일)가 렌더링되는 것을 방지
			if result.UserId != rp.UserId {
				http.NotFound(w, r)
				return
			}
			// Result → Campaign을 통해 TemplateContext 구성
			campaign, err := models.GetCampaign(result.CampaignId, result.UserId)
			if err == nil {
				// (#29) 완료된 캠페인은 RP 본문 송출 차단 — 410 Gone
				if campaign.Status == models.CampaignComplete {
					log.Infof("rp: campaign complete (rp_id=%d, rid=%s, campaign_id=%d)", id, rid, campaign.Id)
					http.Error(w, "Campaign complete", http.StatusGone)
					return
				}
				ptx, err := models.NewPhishingTemplateContext(&campaign, result.BaseRecipient, rid)
				if err == nil {
					pageHTML, _ = models.ExecuteTemplate(rp.HTML, ptx)
				}
			}
		}
	}
	if pageHTML == "" {
		pageHTML = rp.HTML
	}
	pageHTML = ensureMiniSwal(pageHTML)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(pageHTML))
}
