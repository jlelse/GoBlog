package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/carlmjohnson/requests"
	"github.com/jlaffaye/ftp"
	"go.goblog.app/app/pkgs/contenttype"
	"go.goblog.app/app/pkgs/utils"
)

func (a *goBlog) initMediaStorage() {
	a.mediaStorageInit.Do(func() {
		type initFunc func() mediaStorage
		for _, fc := range []initFunc{a.initBunnyCdnMediaStorage, a.initFtpMediaStorage, a.initLocalMediaStorage} {
			a.mediaStorage = fc()
			if a.mediaStorage != nil {
				break
			}
		}
	})
}

func (a *goBlog) mediaStorageEnabled() bool {
	a.initMediaStorage()
	return a.mediaStorage != nil
}

var errNoMediaStorageConfigured = errors.New("no media storage configured")

type mediaStorageSaveFunc func(filename string, file io.Reader) (location string, err error)

func (a *goBlog) saveMediaFile(filename string, f io.Reader) (string, error) {
	a.initMediaStorage()
	if a.mediaStorage == nil {
		return "", errNoMediaStorageConfigured
	}
	loc, err := a.mediaStorage.save(filename, f)
	if err != nil {
		return "", err
	}
	return a.getFullAddress(loc), nil
}

func (a *goBlog) deleteMediaFile(filename string) error {
	a.initMediaStorage()
	if a.mediaStorage == nil {
		return errNoMediaStorageConfigured
	}
	return a.mediaStorage.delete(filepath.Base(filename))
}

type mediaFile struct {
	Name     string
	Location string
	Time     time.Time
	Size     int64
}

func (a *goBlog) mediaFiles() ([]*mediaFile, error) {
	a.initMediaStorage()
	if a.mediaStorage == nil {
		return nil, errNoMediaStorageConfigured
	}
	return a.mediaStorage.files()
}

func (a *goBlog) mediaFileLocation(name string) string {
	a.initMediaStorage()
	if a.mediaStorage == nil {
		return ""
	}
	return a.mediaStorage.location(name)
}

type mediaStorage interface {
	save(filename string, file io.Reader) (location string, err error)
	delete(filename string) (err error)
	files() (files []*mediaFile, err error)
	location(filename string) (location string)
}

type localMediaStorage struct {
	mediaURL string // optional
	path     string // required
}

func (a *goBlog) initLocalMediaStorage() mediaStorage {
	ms := &localMediaStorage{
		path: mediaFilePath,
	}
	if config := a.cfg.Micropub.MediaStorage; config != nil && config.MediaURL != "" {
		ms.mediaURL = config.MediaURL
	}
	return ms
}

func (l *localMediaStorage) save(filename string, file io.Reader) (location string, err error) {
	if err = utils.SaveToFile(file, filepath.Join(l.path, filename)); err != nil {
		return "", err
	}
	return l.location(filename), nil
}

func (l *localMediaStorage) delete(filename string) (err error) {
	if err = os.MkdirAll(l.path, 0777); err != nil {
		return err
	}
	return os.Remove(filepath.Join(l.path, filename))
}

func (l *localMediaStorage) files() (files []*mediaFile, err error) {
	if err = os.MkdirAll(l.path, 0777); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(l.path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		fi, er := e.Info()
		if er != nil {
			continue
		}
		if fi.Mode().IsRegular() {
			files = append(files, &mediaFile{
				Name:     fi.Name(),
				Location: l.location(fi.Name()),
				Time:     fi.ModTime(),
				Size:     fi.Size(),
			})
		}
	}
	return files, nil
}

func (l *localMediaStorage) location(name string) string {
	if l.mediaURL != "" {
		return fmt.Sprintf("%s/%s", l.mediaURL, name)
	}
	return fmt.Sprintf("/m/%s", name)
}

type bunnyMediaStorage struct {
	address    string       // required
	apiKey     string       // required
	mediaURL   string       // required
	httpClient *http.Client // required
}

func (a *goBlog) initBunnyCdnMediaStorage() mediaStorage {
	config := a.cfg.Micropub.MediaStorage
	if config == nil || config.BunnyStorageName == "" || config.BunnyStorageKey == "" || config.MediaURL == "" {
		return nil
	}
	address := "storage.bunnycdn.com"
	if config.BunnyStorageRegion != "" {
		address = fmt.Sprintf("%s.%s", strings.ToLower(config.BunnyStorageRegion), address)
	}
	address = "https://" + address + "/" + config.BunnyStorageName + "/"
	return &bunnyMediaStorage{
		httpClient: a.httpClient,
		mediaURL:   config.MediaURL,
		address:    address,
		apiKey:     config.BunnyStorageKey,
	}
}

type bunnyFileInfo struct {
	ObjectName  string `json:"ObjectName"`
	Length      int64  `json:"Length"`
	LastChanged string `json:"LastChanged"`
	IsDirectory bool   `json:"IsDirectory"`
}

func (f *bunnyMediaStorage) save(filename string, file io.Reader) (location string, err error) {
	err = requests.URL(f.address+filename).
		Method(http.MethodPut).
		Client(f.httpClient).
		Header("AccessKey", f.apiKey).
		Header("Content-Type", "application/octet-stream").
		BodyReader(file).
		Fetch(context.Background())
	if err != nil {
		return "", err
	}
	return f.location(filename), nil
}

func (f *bunnyMediaStorage) delete(filename string) (err error) {
	return requests.URL(f.address+filename).
		Method(http.MethodDelete).
		Client(f.httpClient).
		Header("AccessKey", f.apiKey).
		Fetch(context.Background())
}

func (f *bunnyMediaStorage) files() (files []*mediaFile, err error) {
	var bunnyFiles []bunnyFileInfo
	err = requests.URL(f.address).
		Client(f.httpClient).
		Header("AccessKey", f.apiKey).
		Header("Accept", contenttype.JSON).
		ToJSON(&bunnyFiles).
		Fetch(context.Background())
	if err != nil {
		return nil, err
	}
	for _, s := range bunnyFiles {
		if !s.IsDirectory {
			t, _ := dateparse.ParseIn(s.LastChanged, time.UTC)
			files = append(files, &mediaFile{
				Name:     s.ObjectName,
				Location: f.location(s.ObjectName),
				Time:     t,
				Size:     int64(s.Length),
			})
		}
	}
	return files, nil
}

func (f *bunnyMediaStorage) location(name string) string {
	return fmt.Sprintf("%s/%s", f.mediaURL, name)
}

type ftpMediaStorage struct {
	address  string // required
	user     string // required
	password string // required
	mediaURL string // required
}

func (a *goBlog) initFtpMediaStorage() mediaStorage {
	config := a.cfg.Micropub.MediaStorage
	if config == nil || config.FTPAddress == "" || config.FTPUser == "" || config.FTPPassword == "" {
		return nil
	}
	return &ftpMediaStorage{
		address:  config.FTPAddress,
		user:     config.FTPUser,
		password: config.FTPPassword,
		mediaURL: config.MediaURL,
	}
}

func (f *ftpMediaStorage) save(filename string, file io.Reader) (location string, err error) {
	c, err := f.connection()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = c.Quit()
	}()
	if err = c.Stor(filename, file); err != nil {
		return "", err
	}
	return f.location(filename), nil
}

func (f *ftpMediaStorage) delete(filename string) (err error) {
	c, err := f.connection()
	if err != nil {
		return err
	}
	defer func() {
		_ = c.Quit()
	}()
	return c.Delete(filename)
}

func (f *ftpMediaStorage) files() (files []*mediaFile, err error) {
	c, err := f.connection()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = c.Quit()
	}()
	entries, err := c.List("")
	if err != nil {
		return nil, err
	}
	for _, s := range entries {
		if s.Type == ftp.EntryTypeFile {
			files = append(files, &mediaFile{
				Name:     s.Name,
				Location: f.location(s.Name),
				Time:     s.Time,
				Size:     int64(s.Size),
			})
		}
	}
	return files, nil
}

func (f *ftpMediaStorage) location(name string) string {
	return fmt.Sprintf("%s/%s", f.mediaURL, name)
}

func (f *ftpMediaStorage) connection() (*ftp.ServerConn, error) {
	if f.address == "" || f.user == "" || f.password == "" {
		return nil, errors.New("missing FTP config")
	}
	c, err := ftp.Dial(f.address, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		return nil, err
	}
	if err = c.Login(f.user, f.password); err != nil {
		return nil, err
	}
	return c, nil
}
