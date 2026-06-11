package models

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"bitbucket.org/liamstask/goose/lib/goose"

	"github.com/AegisAX/AegisCampus/auth"
	"github.com/AegisAX/AegisCampus/config"
	mysql "github.com/go-sql-driver/mysql"

	log "github.com/AegisAX/AegisCampus/logger"
	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3" // Blank import needed to import sqlite3
)

var db *gorm.DB
var conf *config.Config

const MaxDatabaseConnectionAttempts int = 10

// DefaultAdminUsername is the default username for the administrative user
const DefaultAdminUsername = "admin"

// InitialAdminPassword is the environment variable that specifies which
// password to use for the initial root login instead of generating one
// randomly
const InitialAdminPassword = "AEGISCAMPUS_INITIAL_ADMIN_PASSWORD"

// InitialAdminApiToken is the environment variable that specifies the
// API token to seed the initial root login instead of generating one
// randomly
const InitialAdminApiToken = "AEGISCAMPUS_INITIAL_ADMIN_API_TOKEN"

const (
	CampaignInProgress     string = "In progress"
	CampaignQueued         string = "Queued"
	CampaignCreated        string = "Created"
	CampaignEmailsSent     string = "Emails Sent"
	CampaignComplete       string = "Completed"
	EventSent              string = "Sent"
	EventSendingError      string = "Error Sending Email"
	EventOpened            string = "Opened"
	EventClicked           string = "Clicked"
	EventDataSubmit        string = "Submitted"
	EventReported          string = "Reported"
	EventProxyRequest      string = "Proxied request"
	EventAttachExecuted    string = "Executed"
	EventTrainingCompleted string = "Trained"
	StatusSuccess          string = "Success"
	StatusQueued           string = "Queued"
	StatusSending          string = "Sending"
	StatusUnknown          string = "Unknown"
	StatusScheduled        string = "Scheduled"
	StatusRetry            string = "Retrying"
	Error                  string = "Error"
)

// Flash is used to hold flash information for use in templates.
type Flash struct {
	Type    string
	Message string
}

// Response contains the attributes found in an API response
type Response struct {
	Message string      `json:"message"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

// Copy of auth.GenerateSecureKey to prevent cyclic import with auth library
func generateSecureKey() string {
	k := make([]byte, 32)
	io.ReadFull(rand.Reader, k)
	return fmt.Sprintf("%x", k)
}

func chooseDBDriver(name, openStr string) goose.DBDriver {
	d := goose.DBDriver{Name: name, OpenStr: openStr}

	switch name {
	case "mysql":
		d.Import = "github.com/go-sql-driver/mysql"
		d.Dialect = &goose.MySqlDialect{}

	// Default database is sqlite3
	default:
		d.Import = "github.com/mattn/go-sqlite3"
		d.Dialect = &goose.Sqlite3Dialect{}
	}

	return d
}

func createTemporaryPassword(u *User) error {
	var temporaryPassword string
	if envPassword := os.Getenv(InitialAdminPassword); envPassword != "" {
		temporaryPassword = envPassword
	} else {
		// This will result in a 16 character password which could be viewed as an
		// inconvenience, but it should be ok for now.
		temporaryPassword = auth.GenerateSecureKey(auth.MinPasswordLength)
	}
	hash, err := auth.GeneratePasswordHash(temporaryPassword)
	if err != nil {
		return err
	}
	u.Hash = hash
	// Anytime a temporary password is created, we will force the user
	// to change their password
	u.PasswordChangeRequired = true
	err = db.Save(u).Error
	if err != nil {
		return err
	}
	log.Infof("Please login with the username admin and the password %s", temporaryPassword)
	return nil
}

// Setup initializes the database and runs any needed migrations.
//
// First, it establishes a connection to the database, then runs any migrations
// newer than the version the database is on.
//
// Once the database is up-to-date, we create an admin user (if needed) that
// has a randomly generated API key and password.
// dedupeVideoProgress removes duplicate video_progresses rows, keeping the
// highest id per natural key (user_id, result_id, video_id), so the 20260528
// unique-index migration can apply on databases that accumulated duplicates
// before that index existed. No-op on a fresh database (table absent) or an
// already-migrated one (no duplicates). The nested subquery form is required
// for MySQL (error 1093) and is also valid on SQLite.
func dedupeVideoProgress() error {
	if !db.HasTable("video_progresses") {
		return nil
	}
	var dupes int64
	if err := db.Raw(`SELECT COUNT(*) FROM (
		SELECT 1 FROM video_progresses
		GROUP BY user_id, result_id, video_id HAVING COUNT(*) > 1
	) d`).Row().Scan(&dupes); err != nil {
		return err
	}
	if dupes == 0 {
		return nil
	}
	log.Warnf("video_progresses: %d duplicate natural-key group(s) found; de-duplicating before unique-index migration", dupes)
	return db.Exec(`DELETE FROM video_progresses WHERE id NOT IN (
		SELECT keep_id FROM (
			SELECT MAX(id) AS keep_id FROM video_progresses
			GROUP BY user_id, result_id, video_id
		) t
	)`).Error
}

func Setup(c *config.Config) error {
	// Setup the package-scoped config
	conf = c
	// Setup the goose configuration
	migrateConf := &goose.DBConf{
		MigrationsDir: conf.MigrationsPath,
		Env:           "production",
		Driver:        chooseDBDriver(conf.DBName, conf.DBPath),
	}
	// Get the latest possible migration
	latest, err := goose.GetMostRecentDBVersion(migrateConf.MigrationsDir)
	if err != nil {
		log.Error(err)
		return err
	}

	// Register certificates for tls encrypted db connections
	if conf.DBSSLCaPath != "" {
		switch conf.DBName {
		case "mysql":
			rootCertPool := x509.NewCertPool()
			pem, err := ioutil.ReadFile(conf.DBSSLCaPath)
			if err != nil {
				log.Error(err)
				return err
			}
			if ok := rootCertPool.AppendCertsFromPEM(pem); !ok {
				log.Error("Failed to append PEM.")
				return err
			}
			mysql.RegisterTLSConfig("ssl_ca", &tls.Config{
				RootCAs: rootCertPool,
			})
			// Default database is sqlite3, which supports no tls, as connection
			// is file based
		default:
		}
	}

	// Open our database connection
	i := 0
	for {
		db, err = gorm.Open(conf.DBName, conf.DBPath)
		if err == nil {
			break
		}
		if err != nil && i >= MaxDatabaseConnectionAttempts {
			log.Error(err)
			return err
		}
		i += 1
		log.Warn("waiting for database to be up...")
		time.Sleep(5 * time.Second)
	}
	db.LogMode(false)
	db.SetLogger(log.Logger)
	db.DB().SetMaxOpenConns(1)
	if err != nil {
		log.Error(err)
		return err
	}
	// Pre-migration safety: 20260528 의 unique-index 마이그레이션은 사전 dedup 없이
	// CREATE UNIQUE INDEX 를 실행하므로, 인덱스 도입 전 쌓인 중복
	// (user_id, result_id, video_id) 행이 있으면 적용에 실패해 업그레이드가 중단된다.
	// goose 는 실패 시 멈춰 후속(교정용) 마이그레이션을 실행할 수 없으므로,
	// 마이그레이션 직전 여기서 중복을 제거한다(과거 마이그레이션 파일은 불변).
	if err = dedupeVideoProgress(); err != nil {
		log.Error(err)
		return err
	}
	// Migrate up to the latest version
	err = goose.RunMigrationsOnDb(migrateConf, migrateConf.MigrationsDir, latest, db.DB())
	if err != nil {
		log.Error(err)
		return err
	}
	// Create the admin user if it doesn't exist
	var userCount int64
	var adminUser User
	db.Model(&User{}).Count(&userCount)
	adminRole, err := GetRoleBySlug(RoleAdmin)
	if err != nil {
		log.Error(err)
		return err
	}
	if userCount == 0 {
		adminUser := User{
			Username:               DefaultAdminUsername,
			Role:                   adminRole,
			RoleID:                 adminRole.ID,
			PasswordChangeRequired: true,
		}

		if envToken := os.Getenv(InitialAdminApiToken); envToken != "" {
			adminUser.ApiKey = envToken
		} else {
			adminUser.ApiKey = auth.GenerateSecureKey(auth.APIKeyLength)
		}

		err = db.Save(&adminUser).Error
		if err != nil {
			log.Error(err)
			return err
		}
	}
	// If this is the first time the user is installing AegisCampus, then we will
	// generate a temporary password for the admin user.
	//
	// We do this here instead of in the block above where the admin is created
	// since there's the chance the user executes AegisCampus and has some kind of
	// error, then tries restarting it. If they didn't grab the password out of
	// the logs, then they would have lost it.
	//
	// By doing the temporary password here, we will regenerate that temporary
	// password until the user is able to reset the admin password.
	if adminUser.Username == "" {
		adminUser, err = GetUserByUsername(DefaultAdminUsername)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	if adminUser.PasswordChangeRequired {
		err = createTemporaryPassword(&adminUser)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return nil
}
