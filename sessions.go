package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/araddon/dateparse"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

var loginSessionsStore, captchaSessionsStore *dbSessionStore

const (
	sessionCreatedOn  = "created"
	sessionModifiedOn = "modified"
	sessionExpiresOn  = "expires"
)

func initSessions() {
	deleteExpiredSessions := func() {
		if _, err := appDb.exec("delete from sessions where expires < @now",
			sql.Named("now", time.Now().Local().String())); err != nil {
			log.Println("Failed to delete expired sessions:", err.Error())
		}
	}
	deleteExpiredSessions()
	hourlyHooks = append(hourlyHooks, deleteExpiredSessions)
	loginSessionsStore = &dbSessionStore{
		codecs: securecookie.CodecsFromPairs(jwtKey()),
		options: &sessions.Options{
			Secure:   httpsConfigured(),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		},
	}
	captchaSessionsStore = &dbSessionStore{
		codecs: securecookie.CodecsFromPairs(jwtKey()),
		options: &sessions.Options{
			Secure:   httpsConfigured(),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int((24 * time.Hour).Seconds()),
		},
	}
}

type dbSessionStore struct {
	options *sessions.Options
	codecs  []securecookie.Codec
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

func (s *dbSessionStore) Save(r *http.Request, w http.ResponseWriter, ss *sessions.Session) error {
	_, err := s.SaveGetCookie(r, w, ss)
	return err
}

func (s *dbSessionStore) SaveGetCookie(r *http.Request, w http.ResponseWriter, ss *sessions.Session) (cookie *http.Cookie, err error) {
	if ss.ID == "" {
		if err = s.insert(ss); err != nil {
			return nil, err
		}
	} else if err = s.save(ss); err != nil {
		return nil, err
	}
	if encoded, err := securecookie.EncodeMulti(ss.Name(), ss.ID, s.codecs...); err != nil {
		return nil, err
	} else {
		cookie = sessions.NewCookie(ss.Name(), encoded, ss.Options)
		http.SetCookie(w, cookie)
		return cookie, nil
	}
}

func (s *dbSessionStore) Delete(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	options := *session.Options
	options.MaxAge = -1
	http.SetCookie(w, sessions.NewCookie(session.Name(), "", &options))
	for k := range session.Values {
		delete(session.Values, k)
	}
	if _, err := appDb.exec("delete from sessions where id = @id", sql.Named("id", session.ID)); err != nil {
		return err
	}
	return nil
}

func (s *dbSessionStore) load(session *sessions.Session) (err error) {
	row, err := appDb.queryRow("select data, created, modified, expires from sessions where id = @id", sql.Named("id", session.ID))
	if err != nil {
		return err
	}
	var data, createdStr, modifiedStr, expiresStr string
	if err = row.Scan(&data, &createdStr, &modifiedStr, &expiresStr); err == sql.ErrNoRows {
		return nil
	} else if err != nil {
		return err
	}
	created, _ := dateparse.ParseLocal(createdStr)
	modified, _ := dateparse.ParseLocal(modifiedStr)
	expires, _ := dateparse.ParseLocal(expiresStr)
	if expires.Before(time.Now()) {
		return errors.New("session expired")
	}
	if err = securecookie.DecodeMulti(session.Name(), data, &session.Values, s.codecs...); err != nil {
		return err
	}
	session.Values[sessionCreatedOn] = created
	session.Values[sessionModifiedOn] = modified
	session.Values[sessionExpiresOn] = expires
	return nil
}

func (s *dbSessionStore) insert(session *sessions.Session) (err error) {
	created := time.Now()
	modified := time.Now()
	expires := time.Now().Add(time.Second * time.Duration(session.Options.MaxAge))
	delete(session.Values, sessionCreatedOn)
	delete(session.Values, sessionExpiresOn)
	delete(session.Values, sessionModifiedOn)
	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, s.codecs...)
	if err != nil {
		return err
	}
	res, err := appDb.exec("insert into sessions(data, created, modified, expires) values(@data, @created, @modified, @expires)",
		sql.Named("data", encoded), sql.Named("created", created.Local().String()), sql.Named("modified", modified.Local().String()), sql.Named("expires", expires.Local().String()))
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
	_, err = appDb.exec("update sessions set data = @data, modified = @modified where id = @id",
		sql.Named("data", encoded), sql.Named("modified", time.Now().Local().String()), sql.Named("id", session.ID))
	if err != nil {
		return err
	}
	return nil
}
