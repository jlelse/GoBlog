package main

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"go.goblog.app/app/pkgs/bufferpool"
)

const (
	sessionCreatedOn  = "created"
	sessionModifiedOn = "modified"
	sessionExpiresOn  = "expires"
)

func (a *goBlog) initSessions() {
	deleteExpiredSessions := func() {
		if _, err := a.db.Exec(
			"delete from sessions where expires < @now",
			sql.Named("now", utcNowString()),
		); err != nil {
			log.Println("Failed to delete expired sessions:", err.Error())
		}
	}
	deleteExpiredSessions()
	a.hourlyHooks = append(a.hourlyHooks, deleteExpiredSessions)
	a.loginSessions = &dbSessionStore{
		options: &sessions.Options{
			Secure:   a.useSecureCookies(),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
			Path:     "/", // Cookie for all pages
		},
		db: a.db,
	}
	a.captchaSessions = &dbSessionStore{
		options: &sessions.Options{
			Secure:   a.useSecureCookies(),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((24 * time.Hour).Seconds()),
			Path:     "/", // Cookie for all pages
		},
		db: a.db,
	}
}

type dbSessionStore struct {
	options *sessions.Options
	db      *database
}

func (s *dbSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

func (s *dbSessionStore) New(r *http.Request, name string) (session *sessions.Session, err error) {
	session = sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	if c, cErr := r.Cookie(name); cErr == nil && strings.HasPrefix(c.Value, session.Name()+"-") {
		// Has cookie, load from database
		session.ID = c.Value
		if s.load(session) != nil {
			// Failed to load session from database, delete the ID (= new session)
			session.ID = ""
		}
	}
	// If no ID, the session is new
	session.IsNew = session.ID == ""
	return session, err
}

func (s *dbSessionStore) Save(_ *http.Request, w http.ResponseWriter, ss *sessions.Session) (err error) {
	if ss.ID == "" {
		// Is new session, save it to database
		if err = s.insert(ss); err != nil {
			return err
		}
	} else {
		// Update existing session
		if err = s.save(ss); err != nil {
			return err
		}
	}
	http.SetCookie(w, sessions.NewCookie(ss.Name(), ss.ID, ss.Options))
	return nil
}

func (s *dbSessionStore) Delete(_ *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	options := *session.Options
	options.MaxAge = -1
	http.SetCookie(w, sessions.NewCookie(session.Name(), "", &options))
	for k := range session.Values {
		delete(session.Values, k)
	}
	if _, err := s.db.Exec("delete from sessions where id = @id", sql.Named("id", session.ID)); err != nil {
		return err
	}
	return nil
}

func (s *dbSessionStore) load(session *sessions.Session) (err error) {
	row, err := s.db.QueryRow(
		"select data, created, modified, expires from sessions where id = @id and expires > @now",
		sql.Named("id", session.ID),
		sql.Named("now", utcNowString()),
	)
	if err != nil {
		return err
	}
	var createdStr, modifiedStr, expiresStr string
	var data []byte
	if err = row.Scan(&data, &createdStr, &modifiedStr, &expiresStr); err != nil {
		return err
	}
	if err = gob.NewDecoder(bytes.NewReader(data)).Decode(&session.Values); err != nil {
		return err
	}
	session.Values[sessionCreatedOn] = noError(dateparse.ParseLocal(createdStr))
	session.Values[sessionModifiedOn] = noError(dateparse.ParseLocal(modifiedStr))
	session.Values[sessionExpiresOn] = noError(dateparse.ParseLocal(expiresStr))
	return nil
}

func (s *dbSessionStore) insert(session *sessions.Session) (err error) {
	deleteSessionValuesNotNeededForDb(session)
	encoded := bufferpool.Get()
	defer bufferpool.Put(encoded)
	if err := gob.NewEncoder(encoded).Encode(session.Values); err != nil {
		return err
	}
	session.ID = session.Name() + "-" + uuid.NewString()
	created, modified := utcNowString(), utcNowString()
	expires := time.Now().UTC().Add(time.Second * time.Duration(session.Options.MaxAge)).Format(time.RFC3339)
	_, err = s.db.Exec(
		"insert or replace into sessions(id, data, created, modified, expires) values(@id, @data, @created, @modified, @expires)",
		sql.Named("id", session.ID),
		sql.Named("data", encoded.Bytes()),
		sql.Named("created", created),
		sql.Named("modified", modified),
		sql.Named("expires", expires),
	)
	return err
}

func (s *dbSessionStore) save(session *sessions.Session) (err error) {
	if session.IsNew {
		return s.insert(session)
	}
	deleteSessionValuesNotNeededForDb(session)
	encoded := bufferpool.Get()
	defer bufferpool.Put(encoded)
	if err = gob.NewEncoder(encoded).Encode(session.Values); err != nil {
		return err
	}
	_, err = s.db.Exec(
		"update sessions set data = @data, modified = @modified where id = @id",
		sql.Named("data", encoded.Bytes()),
		sql.Named("modified", utcNowString()),
		sql.Named("id", session.ID),
	)
	if err != nil {
		return err
	}
	return nil
}

func deleteSessionValuesNotNeededForDb(session *sessions.Session) {
	delete(session.Values, sessionCreatedOn)
	delete(session.Values, sessionExpiresOn)
	delete(session.Values, sessionModifiedOn)
}
