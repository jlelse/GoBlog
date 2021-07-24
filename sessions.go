package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/araddon/dateparse"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

const (
	sessionCreatedOn  = "created"
	sessionModifiedOn = "modified"
	sessionExpiresOn  = "expires"
)

func (a *goBlog) initSessions() {
	deleteExpiredSessions := func() {
		if _, err := a.db.exec(
			"delete from sessions where expires < @now",
			sql.Named("now", utcNowString()),
		); err != nil {
			log.Println("Failed to delete expired sessions:", err.Error())
		}
	}
	deleteExpiredSessions()
	a.hourlyHooks = append(a.hourlyHooks, deleteExpiredSessions)
	a.loginSessions = &dbSessionStore{
		codecs: securecookie.CodecsFromPairs([]byte(a.cfg.Server.JWTSecret)),
		options: &sessions.Options{
			Secure:   a.httpsConfigured(),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
			Path:     "/", // Cookie for all pages
		},
		db: a.db,
	}
	a.captchaSessions = &dbSessionStore{
		codecs: securecookie.CodecsFromPairs([]byte(a.cfg.Server.JWTSecret)),
		options: &sessions.Options{
			Secure:   a.httpsConfigured(),
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
	codecs  []securecookie.Codec
	db      *database
}

func (s *dbSessionStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(s, name)
}

func (s *dbSessionStore) New(r *http.Request, name string) (session *sessions.Session, err error) {
	session = sessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true
	if cook, errCookie := r.Cookie(name); errCookie == nil {
		if err = securecookie.DecodeMulti(name, cook.Value, &session.ID, s.codecs...); err == nil {
			session.IsNew = s.load(session) == nil
		}
	}
	return session, err
}

func (s *dbSessionStore) Save(r *http.Request, w http.ResponseWriter, ss *sessions.Session) (err error) {
	if ss.ID == "" {
		if err = s.insert(ss); err != nil {
			return err
		}
	} else if err = s.save(ss); err != nil {
		return err
	}
	if encoded, err := securecookie.EncodeMulti(ss.Name(), ss.ID, s.codecs...); err != nil {
		return err
	} else {
		http.SetCookie(w, sessions.NewCookie(ss.Name(), encoded, ss.Options))
		return nil
	}
}

func (s *dbSessionStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	options := *session.Options
	options.MaxAge = -1
	http.SetCookie(w, sessions.NewCookie(session.Name(), "", &options))
	for k := range session.Values {
		delete(session.Values, k)
	}
	if _, err := s.db.exec("delete from sessions where id = @id", sql.Named("id", session.ID)); err != nil {
		return err
	}
	return nil
}

func (s *dbSessionStore) load(session *sessions.Session) (err error) {
	row, err := s.db.queryRow("select data, created, modified, expires from sessions where id = @id and expires > @now", sql.Named("id", session.ID), sql.Named("now", utcNowString()))
	if err != nil {
		return err
	}
	var data, createdStr, modifiedStr, expiresStr string
	if err = row.Scan(&data, &createdStr, &modifiedStr, &expiresStr); err != nil {
		return err
	}
	created, _ := dateparse.ParseLocal(createdStr)
	modified, _ := dateparse.ParseLocal(modifiedStr)
	expires, _ := dateparse.ParseLocal(expiresStr)
	if err = securecookie.DecodeMulti(session.Name(), data, &session.Values, s.codecs...); err != nil {
		return err
	}
	session.Values[sessionCreatedOn] = created
	session.Values[sessionModifiedOn] = modified
	session.Values[sessionExpiresOn] = expires
	return nil
}

func (s *dbSessionStore) insert(session *sessions.Session) (err error) {
	created := time.Now().UTC()
	modified := time.Now().UTC()
	expires := time.Now().UTC().Add(time.Second * time.Duration(session.Options.MaxAge))
	delete(session.Values, sessionCreatedOn)
	delete(session.Values, sessionExpiresOn)
	delete(session.Values, sessionModifiedOn)
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.codecs...)
	if err != nil {
		return err
	}
	res, err := s.db.exec("insert or replace into sessions(data, created, modified, expires) values(@data, @created, @modified, @expires)",
		sql.Named("data", encoded), sql.Named("created", created.Format(time.RFC3339)), sql.Named("modified", modified.Format(time.RFC3339)), sql.Named("expires", expires.Format(time.RFC3339)))
	if err != nil {
		return err
	}
	lastInserted, err := res.LastInsertId()
	if err != nil {
		return err
	}
	session.ID = fmt.Sprintf("%d", lastInserted)
	return nil
}

func (s *dbSessionStore) save(session *sessions.Session) (err error) {
	if session.IsNew {
		return s.insert(session)
	}
	delete(session.Values, sessionCreatedOn)
	delete(session.Values, sessionExpiresOn)
	delete(session.Values, sessionModifiedOn)
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.codecs...)
	if err != nil {
		return err
	}
	_, err = s.db.exec("update sessions set data = @data, modified = @modified where id = @id",
		sql.Named("data", encoded), sql.Named("modified", utcNowString()), sql.Named("id", session.ID))
	if err != nil {
		return err
	}
	return nil
}
