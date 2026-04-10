package controllers

import (
    "compress/gzip"
    "context"
    "crypto/sha256"
    "crypto/tls"
    "encoding/hex"
    "io"
    "os"
    "path/filepath"
    "strconv"
    "html/template"
    "net/http"
    "net/url"
    "strings"
    "time"

    "github.com/NYTimes/gziphandler"
    "github.com/gophish/gophish/auth"
    "github.com/gophish/gophish/config"
    ctx "github.com/gophish/gophish/context"
    "github.com/gophish/gophish/controllers/api"
    log "github.com/gophish/gophish/logger"
    mid "github.com/gophish/gophish/middleware"
    "github.com/gophish/gophish/middleware/ratelimit"
    "github.com/gophish/gophish/models"
    "github.com/gophish/gophish/util"
    "github.com/gophish/gophish/worker"
    "github.com/gorilla/csrf"
    "github.com/gorilla/handlers"
    "github.com/gorilla/mux"
    "github.com/gorilla/sessions"
    "github.com/jordan-wright/unindexed"
)

// AdminServerOption is a functional option that is used to configure the
// admin server
type AdminServerOption func(*AdminServer)

// AdminServer is an HTTP server that implements the administrative Gophish
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
		ReadTimeout: 10 * time.Second,
		Addr:        config.ListenURL,
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
	router.HandleFunc("/videos/upload", mid.Use(as.UploadVideo, mid.RequireLogin)).Methods("POST")
	router.HandleFunc("/videos/thumb/{id:[0-9]+}", as.HandleVideoThumb).Methods("GET")
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
	router.PathPrefix("/").Handler(http.FileServer(unindexed.Dir("./static/")))

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
// and management of user accounts within Gophish.
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
// required either by the Gophish system or an administrator.
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

// GET /videos/thumb/{id} : 썸네일 이미지 서빙 (로그인 없이 접근 가능)
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

	// 썸네일 경로가 정해둔 베이스 하위인지 검증
	baseAbs, _ := filepath.Abs(filepath.Join("static", "videos"))
	if !util.IsUnderBaseDir(baseAbs, v.ThumbnailPath) {
		http.NotFound(w, r)
		return
	}

	// 파일 존재 확인 후 서빙
	if _, err := os.Stat(v.ThumbnailPath); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")           // 생성은 jpg
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, v.ThumbnailPath)
}

// ===== UI 업로드 핸들러 (세션 + CSRF) =====
func (as *AdminServer) UploadVideo(w http.ResponseWriter, r *http.Request) {
    // 로그인 사용자 (Admin 라우트는 user가 항상 세팅됨)
    userId := int64(0)
    if u, ok := ctx.Get(r, "user").(models.User); ok && u.Id != 0 {
        userId = u.Id
    } else if v := ctx.Get(r, "user_id"); v != nil { // 혹시 모를 보조 경로
        if vv, ok := v.(int64); ok { userId = vv }
    }
    if userId == 0 {
        api.JSONResponse(w, models.Response{Success:false, Message:"unauthorized"}, http.StatusUnauthorized)
        return
    }
    if err := r.ParseMultipartForm(1 << 30); err != nil {
        api.JSONResponse(w, models.Response{Success:false, Message:"Parse error"}, http.StatusBadRequest)
        return
    }
    file, handler, err := r.FormFile("file")
    if err != nil {
        api.JSONResponse(w, models.Response{Success:false, Message:"File required"}, http.StatusBadRequest)
        return
    }
    defer file.Close()

    name := strings.TrimSpace(r.FormValue("name"))
    if name == "" {
        base := strings.TrimSuffix(filepath.Base(handler.Filename), filepath.Ext(handler.Filename))
        name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(base, "_", " "), "-", " "))
    }
    description := r.FormValue("description")

    if err := os.MkdirAll(util.VideoStorageDirAbs, 0755); err != nil {
        log.Error(err)
        api.JSONResponse(w, models.Response{Success:false, Message:"Storage error"}, http.StatusInternalServerError)
        return
    }

    // 임시 저장 + 해시
	tmpFile, err := os.CreateTemp(util.VideoStorageDirAbs, "upload-*")
	if err != nil {
		log.Error(err)
		api.JSONResponse(w, models.Response{Success:false, Message:"Create temp file error"}, http.StatusInternalServerError)
		return
	}
	tmpName := tmpFile.Name()
	cleanupTmp := true
	defer func() { if cleanupTmp {_ = os.Remove(tmpName)} }()
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpFile, hasher), file); err != nil {
		tmpFile.Close()
		api.JSONResponse(w, models.Response{Success:false, Message:"Write file error"}, http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

    sumHex := hex.EncodeToString(hasher.Sum(nil))
    ext := strings.ToLower(filepath.Ext(handler.Filename))
    finalName := sumHex + ext
    finalPath := filepath.Join(util.VideoStorageDirAbs, finalName)

    // 중복 처리
	if _, err := os.Stat(finalPath); err == nil {
	} else {
		if err := os.Rename(tmpFile.Name(), finalPath); err != nil {
			in, err1 := os.Open(tmpName)
			if err1 != nil {
				api.JSONResponse(w, models.Response{Success:false, Message:"Finalize file error"}, http.StatusInternalServerError)
				return
			}
			out, err2 := os.Create(finalPath)
			if err2 != nil {
				in.Close()
				api.JSONResponse(w, models.Response{Success:false, Message:"Finalize file error"}, http.StatusInternalServerError)
				return
			}
			if _, err := io.Copy(out, in); err != nil {
				out.Close(); in.Close()
				_ = os.Remove(finalPath)
				api.JSONResponse(w, models.Response{Success:false, Message:"Finalize file error"}, http.StatusInternalServerError)
				return
			}
			out.Close(); in.Close()
		} else {
			cleanupTmp = false
		}
	}

    // 길이
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

    // 썸네일
    thumbDir := filepath.Join(util.VideoStorageDirAbs, "thumbs")
    thumbName := sumHex + ".jpg"
    thumbPath := filepath.Join(thumbDir, thumbName)
    if err := util.GenerateThumbnail(finalPath, thumbPath, 1, 480); err != nil {
        log.Errorf("thumbnail generation failed: %v", err)
        thumbPath = "" // 실패해도 업로드는 성공시킴
    }

    v := &models.Video{
        UserId:          userId,
        Name:            name,
        Description:     description,
        FileName:        finalName,
        FilePath:        finalPath,
        ThumbnailPath:   thumbPath,
        DurationSeconds: durationSeconds,
        IsPublic:        false, // UI 업로드는 공개 여부 입력이 없으니 기본 false
    }
    if err := models.CreateVideo(v); err != nil {
        api.JSONResponse(w, models.Response{Success:false, Message:"DB save error"}, http.StatusInternalServerError)
        return
    }
    api.JSONResponse(w, v, http.StatusCreated)
}

