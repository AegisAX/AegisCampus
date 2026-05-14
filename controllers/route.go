package controllers

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/AegisAX/Sentinel/auth"
	"github.com/AegisAX/Sentinel/config"
	ctx "github.com/AegisAX/Sentinel/context"
	"github.com/AegisAX/Sentinel/controllers/api"
	log "github.com/AegisAX/Sentinel/logger"
	mid "github.com/AegisAX/Sentinel/middleware"
	"github.com/AegisAX/Sentinel/middleware/ratelimit"
	"github.com/AegisAX/Sentinel/models"
	"github.com/AegisAX/Sentinel/util"
	"github.com/AegisAX/Sentinel/worker"
	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/AegisAX/Sentinel/middleware"
)

// AdminServerOption is a functional option that is used to configure the
// admin server
type AdminServerOption func(*AdminServer)

// AdminServer is an HTTP server that implements the administrative Sentinel
// handlers, including the dashboard and REST API.
type AdminServer struct {
	server  *http.Server
	worker  worker.Worker
	config  config.AdminServer
	limiter *ratelimit.PostLimiter
}

var defaultTLSConfig = &tls.Config{
	PreferServerCipherSuites: true,
	CurvePreferences: []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
	},
	MinVersion: tls.VersionTLS12,
	CipherSuites: []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

		// Kept for backwards compatibility with some clients
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	},
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) AdminServerOption {
	return func(as *AdminServer) {
		as.worker = w
	}
}

// NewAdminServer returns a new instance of the AdminServer with the
// provided config and options applied.
func NewAdminServer(config config.AdminServer, options ...AdminServerOption) *AdminServer {
	defaultWorker, _ := worker.New()
	defaultServer := &http.Server{
		// ReadTimeout을 제거하고 ReadHeaderTimeout만 유지합니다.
		// ReadTimeout은 요청 헤더 + 바디 전체를 읽는 시간을 제한하므로
		// 대용량 영상 업로드 시 연결이 강제 종료됩니다.
		// ReadHeaderTimeout은 헤더만 제한하여 업로드에 영향을 주지 않으면서
		// Slowloris 공격을 방어합니다.
		ReadHeaderTimeout: 10 * time.Second,
		Addr:              config.ListenURL,
	}
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &AdminServer{
		worker:  defaultWorker,
		server:  defaultServer,
		limiter: defaultLimiter,
		config:  config,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// Start launches the admin server, listening on the configured address.
func (as *AdminServer) Start() {
	if as.worker != nil {
		go as.worker.Start()
	}
	if as.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		as.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(as.config.CertPath, as.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting admin server at https://%s", as.config.ListenURL)
		log.Fatal(as.server.ListenAndServeTLS(as.config.CertPath, as.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting admin server at http://%s", as.config.ListenURL)
	log.Fatal(as.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (as *AdminServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return as.server.Shutdown(ctx)
}

// SetupAdminRoutes creates the routes for handling requests to the web interface.
// This function returns an http.Handler to be used in http.ListenAndServe().
func (as *AdminServer) registerRoutes() {
	router := mux.NewRouter()
	// Base Front-end routes
	router.HandleFunc("/", mid.Use(as.Base, mid.RequireLogin))
	router.HandleFunc("/login", mid.Use(as.Login, as.limiter.Limit))
	router.HandleFunc("/logout", mid.Use(as.Logout, mid.RequireLogin))
	router.HandleFunc("/reset_password", mid.Use(as.ResetPassword, mid.RequireLogin))
	router.HandleFunc("/campaigns", mid.Use(as.Campaigns, mid.RequireLogin))
	router.HandleFunc("/campaigns/{id:[0-9]+}", mid.Use(as.CampaignID, mid.RequireLogin))
	router.HandleFunc("/templates", mid.Use(as.Templates, mid.RequireLogin))
	router.HandleFunc("/groups", mid.Use(as.Groups, mid.RequireLogin))
	router.HandleFunc("/landing_pages", mid.Use(as.LandingPages, mid.RequireLogin))
	router.HandleFunc("/redirect_pages", mid.Use(as.RedirectPages, mid.RequireLogin))
	router.HandleFunc("/videos", mid.Use(as.Videos, mid.RequireLogin))
	router.HandleFunc("/videos/stream/{id:[0-9]+}", mid.Use(as.StreamVideo, mid.RequireLogin))
	// 향후 Phase 6 에서 /api/videos/ POST 로 통일 예정 (다른 객체들과 라우팅 패턴 일치).
	// 그 전까지 defense-in-depth 로 ModifyObjects 권한을 명시 검사한다.
	// 현재 RBAC 모델에서는 admin/user 둘 다 이 권한을 가지므로 동작 변화는 없으나,
	// 향후 view-only 같은 새 역할이 추가될 때 자동으로 차단된다.
	router.HandleFunc("/videos/upload", mid.Use(as.UploadVideo, mid.RequirePermission(models.PermissionModifyObjects), mid.RequireLogin)).Methods("POST")
	router.HandleFunc("/videos/thumb/{id:[0-9]+}", mid.Use(as.HandleVideoThumb, mid.RequireLogin)).Methods("GET")
	router.HandleFunc("/media/{id:[0-9]+}", mid.Use(as.StreamVideo, mid.RequireLogin)).Methods("GET")
	router.HandleFunc("/sending_profiles", mid.Use(as.SendingProfiles, mid.RequireLogin))
	router.HandleFunc("/settings", mid.Use(as.Settings, mid.RequireLogin))
	router.HandleFunc("/users", mid.Use(as.UserManagement, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/webhooks", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/impersonate", mid.Use(as.Impersonate, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	// Create the API routes
	api := api.NewServer(
		api.WithWorker(as.worker),
		api.WithLimiter(as.limiter),
	)
	router.PathPrefix("/api/").Handler(api)

	// Setup static file serving
	router.PathPrefix("/").Handler(http.FileServer(middleware.NoIndexDir("./static/")))

	// Setup CSRF Protection
	csrfKey := []byte(as.config.CSRFKey)
	if len(csrfKey) == 0 {
		csrfKey = []byte(auth.GenerateSecureKey(auth.APIKeyLength))
	}
	csrfHandler := csrf.Protect(csrfKey,
		csrf.FieldName("csrf_token"),
		csrf.Secure(as.config.UseTLS),
		csrf.TrustedOrigins(as.config.TrustedOrigins))
	adminHandler := csrfHandler(router)
	adminHandler = mid.Use(adminHandler.ServeHTTP, mid.CSRFExceptions, mid.GetContext, mid.ApplySecurityHeaders)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	adminHandler = gzipWrapper(adminHandler)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	adminHandler = handlers.ProxyHeaders(adminHandler)

	// Setup logging
	adminHandler = handlers.CombinedLoggingHandler(log.Writer(), adminHandler)
	as.server.Handler = adminHandler
}

type templateParams struct {
	Title        string
	Flashes      []interface{}
	User         models.User
	Token        string
	Version      string
	ModifySystem bool
}

// newTemplateParams returns the default template parameters for a user and
// the CSRF token.
func newTemplateParams(r *http.Request) templateParams {
	user := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	modifySystem, _ := user.HasPermission(models.PermissionModifySystem)
	return templateParams{
		Token:        csrf.Token(r),
		User:         user,
		ModifySystem: modifySystem,
		Version:      config.Version,
		Flashes:      session.Flashes(),
	}
}

// Base handles the default path and template execution
func (as *AdminServer) Base(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Dashboard"
	getTemplate(w, "dashboard").ExecuteTemplate(w, "base", params)
}

// Campaigns handles the default path and template execution
func (as *AdminServer) Campaigns(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaigns"
	getTemplate(w, "campaigns").ExecuteTemplate(w, "base", params)
}

// CampaignID handles the default path and template execution
func (as *AdminServer) CampaignID(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Campaign Results"
	getTemplate(w, "campaign_results").ExecuteTemplate(w, "base", params)
}

// Templates handles the default path and template execution
func (as *AdminServer) Templates(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Email Templates"
	getTemplate(w, "templates").ExecuteTemplate(w, "base", params)
}

// Groups handles the default path and template execution
func (as *AdminServer) Groups(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Users & Groups"
	getTemplate(w, "groups").ExecuteTemplate(w, "base", params)
}

// LandingPages handles the default path and template execution
func (as *AdminServer) LandingPages(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Landing Pages"
	getTemplate(w, "landing_pages").ExecuteTemplate(w, "base", params)
}

// RedirectPages handles the Redirect Page management admin page
func (as *AdminServer) RedirectPages(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Redirect Pages"
	getTemplate(w, "redirect_pages").ExecuteTemplate(w, "base", params)
}

// Videos handles the Video Management admin page
func (as *AdminServer) Videos(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Video Management"
	getTemplate(w, "videos").ExecuteTemplate(w, "base", params)
}

// SendingProfiles handles the default path and template execution
func (as *AdminServer) SendingProfiles(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Sending Profiles"
	getTemplate(w, "sending_profiles").ExecuteTemplate(w, "base", params)
}

// Settings handles the changing of settings
func (as *AdminServer) Settings(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		params := newTemplateParams(r)
		params.Title = "Settings"
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Save(r, w)
		getTemplate(w, "settings").ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		u := ctx.Get(r, "user").(models.User)
		currentPw := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_new_password")
		// Check the current password
		err := auth.ValidatePassword(currentPw, u.Hash)
		msg := models.Response{Success: true, Message: "Settings Updated Successfully"}
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		u.Hash = string(newHash)
		if err = models.PutUser(&u); err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusInternalServerError)
			return
		}
		api.JSONResponse(w, msg, http.StatusOK)
	}
}

// UserManagement is an admin-only handler that allows for the registration
// and management of user accounts within Sentinel.
func (as *AdminServer) UserManagement(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "User Management"
	getTemplate(w, "users").ExecuteTemplate(w, "base", params)
}

func (as *AdminServer) nextOrIndex(w http.ResponseWriter, r *http.Request) {
	next := "/"
	url, err := url.Parse(r.FormValue("next"))
	if err == nil {
		path := url.EscapedPath()
		if path != "" {
			next = "/" + strings.TrimLeft(path, "/")
		}
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (as *AdminServer) handleInvalidLogin(w http.ResponseWriter, r *http.Request, message string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	Flash(w, r, "danger", message)
	params := struct {
		User    models.User
		Title   string
		Flashes []interface{}
		Token   string
	}{Title: "Login", Token: csrf.Token(r)}
	params.Flashes = session.Flashes()
	session.Save(r, w)
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	// w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	template.Must(templates, err).ExecuteTemplate(w, "base", params)
}

// Webhooks is an admin-only handler that handles webhooks
func (as *AdminServer) Webhooks(w http.ResponseWriter, r *http.Request) {
	params := newTemplateParams(r)
	params.Title = "Webhooks"
	getTemplate(w, "webhooks").ExecuteTemplate(w, "base", params)
}

// Impersonate allows an admin to login to a user account without needing the password
func (as *AdminServer) Impersonate(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		username := r.FormValue("username")
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Values["id"] = u.Id
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// Login handles the authentication flow for a user. If credentials are valid,
// a session is created
func (as *AdminServer) Login(w http.ResponseWriter, r *http.Request) {
	params := struct {
		User    models.User
		Title   string
		Flashes []interface{}
		Token   string
	}{Title: "Login", Token: csrf.Token(r)}
	session := ctx.Get(r, "session").(*sessions.Session)
	switch {
	case r.Method == "GET":
		params.Flashes = session.Flashes()
		session.Save(r, w)
		templates := template.New("template")
		_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
		if err != nil {
			log.Error(err)
		}
		template.Must(templates, err).ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		// Find the user with the provided username
		username, password := r.FormValue("username"), r.FormValue("password")
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			as.handleInvalidLogin(w, r, "Invalid Username/Password")
			return
		}
		// Validate the user's password
		err = auth.ValidatePassword(password, u.Hash)
		if err != nil {
			log.Error(err)
			as.handleInvalidLogin(w, r, "Invalid Username/Password")
			return
		}
		if u.AccountLocked {
			as.handleInvalidLogin(w, r, "Account Locked")
			return
		}
		u.LastLogin = time.Now().UTC()
		err = models.PutUser(&u)
		if err != nil {
			log.Error(err)
		}
		// If we've logged in, save the session and redirect to the dashboard
		session.Values["id"] = u.Id
		session.Save(r, w)
		as.nextOrIndex(w, r)
	}
}

// Logout destroys the current user session
func (as *AdminServer) Logout(w http.ResponseWriter, r *http.Request) {
	session := ctx.Get(r, "session").(*sessions.Session)
	delete(session.Values, "id")
	Flash(w, r, "success", "You have successfully logged out")
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ResetPassword handles the password reset flow when a password change is
// required either by the Sentinel system or an administrator.
//
// This handler is meant to be used when a user is required to reset their
// password, not just when they want to.
//
// This is an important distinction since in this handler we don't require
// the user to re-enter their current password, as opposed to the flow
// through the settings handler.
//
// To that end, if the user doesn't require a password change, we will
// redirect them to the settings page.
func (as *AdminServer) ResetPassword(w http.ResponseWriter, r *http.Request) {
	u := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	if !u.PasswordChangeRequired {
		Flash(w, r, "info", "Please reset your password through the settings page")
		session.Save(r, w)
		http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
		return
	}
	params := newTemplateParams(r)
	params.Title = "Reset Password"
	switch {
	case r.Method == http.MethodGet:
		params.Flashes = session.Flashes()
		session.Save(r, w)
		getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
		return
	case r.Method == http.MethodPost:
		newPassword := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusBadRequest)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		u.PasswordChangeRequired = false
		u.Hash = newHash
		if err = models.PutUser(&u); err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusInternalServerError)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		// TODO: We probably want to flash a message here that the password was
		// changed successfully. The problem is that when the user resets their
		// password on first use, they will see two flashes on the dashboard-
		// one for their password reset, and one for the "no campaigns created".
		//
		// The solution to this is to revamp the empty page to be more useful,
		// like a wizard or something.
		as.nextOrIndex(w, r)
	}
}

// TODO: Make this execute the template, too
func getTemplate(w http.ResponseWriter, tmpl string) *template.Template {
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/base.html", "templates/nav.html", "templates/"+tmpl+".html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	return template.Must(templates, err)
}

// Flash handles the rendering flash messages
func Flash(w http.ResponseWriter, r *http.Request, t string, m string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	session.AddFlash(models.Flash{
		Type:    t,
		Message: m,
	})
}

// StreamVideo serves a video file with Range support for preview/streaming.
// GET /videos/stream/{id}
func (as *AdminServer) StreamVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, _ := strconv.ParseInt(idStr, 10, 64)

	v, err := models.GetVideo(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	path := v.FilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(util.VideoStorageDirAbs, filepath.Base(path))
	}
	// IsUnderBaseDir로 경로 순회 방어 (HandleVideoThumb과 동일 패턴)
	if !util.IsUnderBaseDir(util.VideoStorageDirAbs, path) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "video/mp4") // 필요시 확장자 기반으로 변경
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, filepath.Base(v.FilePath), fi.ModTime(), f)
}

// GET /videos/thumb/{id} : 썸네일 이미지 서빙 (로그인 필요 — Phase 2 #8)
func (as *AdminServer) HandleVideoThumb(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
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
	// 경로 순회 방어
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

// ===== UI 업로드 핸들러 (세션 + CSRF) =====
func (as *AdminServer) UploadVideo(w http.ResponseWriter, r *http.Request) {
	// 세션 기반 userId 추출 (Admin 라우트 전용 방식 유지)
	userId := int64(0)
	if u, ok := ctx.Get(r, "user").(models.User); ok && u.Id != 0 {
		userId = u.Id
	} else if v := ctx.Get(r, "user_id"); v != nil {
		if vv, ok := v.(int64); ok {
			userId = vv
		}
	}
	if userId == 0 {
		api.JSONResponse(w, models.Response{Success: false, Message: "unauthorized"}, http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, util.MaxVideoUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if err.Error() == "http: request body too large" {
			api.JSONResponse(w, models.Response{
				Success: false,
				Message: fmt.Sprintf("파일 크기가 허용 한도를 초과합니다 (최대 %dMB)", util.MaxVideoUploadBytes>>20),
			}, http.StatusRequestEntityTooLarge)
			return
		}
		api.JSONResponse(w, models.Response{Success: false, Message: "Parse error"}, http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err != nil {
		api.JSONResponse(w, models.Response{Success: false, Message: "File required"}, http.StatusBadRequest)
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
		api.JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}

	// 상대경로로 저장 (이식성 확보)
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
		api.JSONResponse(w, models.Response{Success: false, Message: "DB save error"}, http.StatusInternalServerError)
		return
	}
	api.JSONResponse(w, v, http.StatusCreated)
}
