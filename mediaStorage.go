package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
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

type mediaStorageSaveFunc func(filename string, file io.Reader) (location string, err error)

func (a *goBlog) saveMediaFile(filename string, f io.Reader) (string, error) {
	a.initMediaStorage()
	if a.mediaStorage == nil {
		return "", errors.New("no media storage configured")
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
		return errors.New("no media storage configured")
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
		return nil, errors.New("no media storage configured")
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
	if err = os.MkdirAll(l.path, 0644); err != nil {
		return "", err
	}
	newFile, err := os.Create(filepath.Join(l.path, filename))
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(newFile, file); err != nil {
		return "", err
	}
	return l.location(filename), nil
}

func (l *localMediaStorage) delete(filename string) (err error) {
	if err = os.MkdirAll(l.path, 0644); err != nil {
		return err
	}
	return os.Remove(filepath.Join(l.path, filename))
}

func (l *localMediaStorage) files() (files []*mediaFile, err error) {
	if err = os.MkdirAll(l.path, 0644); err != nil {
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
				Size:     int64(fi.Size()),
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

func (a *goBlog) initBunnyCdnMediaStorage() mediaStorage {
	config := a.cfg.Micropub.MediaStorage
	if config == nil || config.BunnyStorageName == "" || config.BunnyStorageKey == "" || config.MediaURL == "" {
		return nil
	}
	address := "storage.bunnycdn.com:21"
	if config.BunnyStorageRegion != "" {
		address = fmt.Sprintf("%s.%s", strings.ToLower(config.BunnyStorageRegion), address)
	}
	return &ftpMediaStorage{
		address:  address,
		user:     config.BunnyStorageName,
		password: config.BunnyStorageKey,
		mediaURL: config.MediaURL,
	}
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
	defer func() {
		if c != nil {
			_ = c.Quit()
		}
	}()
	if err != nil {
		return "", err
	}
	if err = c.Stor(filename, file); err != nil {
		return "", err
	}
	return f.location(filename), nil
}

func (f *ftpMediaStorage) delete(filename string) (err error) {
	c, err := f.connection()
	defer func() {
		if c != nil {
			_ = c.Quit()
		}
	}()
	if err != nil {
		return err
	}
	return c.Delete(filename)
}

func (f *ftpMediaStorage) files() (files []*mediaFile, err error) {
	c, err := f.connection()
	defer func() {
		if c != nil {
			_ = c.Quit()
		}
	}()
	if err != nil {
		return nil, err
	}
	w := c.Walk("")
	for w.Next() {
		if s := w.Stat(); s.Type == ftp.EntryTypeFile {
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
