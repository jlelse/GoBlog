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

type mediaStorageSaveFunc func(filename string, file io.Reader) (location string, err error)

func (a *goBlog) saveMediaFile(filename string, f io.Reader) (string, error) {
	a.mediaStorageInit.Do(func() {
		type initFunc func() mediaStorage
		for _, fc := range []initFunc{a.initBunnyCdnMediaStorage, a.initFtpMediaStorage, a.initLocalMediaStorage} {
			a.mediaStorage = fc()
			if a.mediaStorage != nil {
				break
			}
		}
	})
	if a.mediaStorage == nil {
		return "", errors.New("no media storage configured")
	}
	loc, err := a.mediaStorage.save(filename, f)
	if err != nil {
		return "", err
	}
	return a.getFullAddress(loc), nil
}

type mediaStorage interface {
	save(filename string, file io.Reader) (location string, err error)
}

type localMediaStorage struct {
	mediaURL string // optional
}

func (a *goBlog) initLocalMediaStorage() mediaStorage {
	ms := &localMediaStorage{}
	if config := a.cfg.Micropub.MediaStorage; config != nil && config.MediaURL != "" {
		ms.mediaURL = config.MediaURL
	}
	return ms
}

func (l *localMediaStorage) save(filename string, file io.Reader) (location string, err error) {
	if err = os.MkdirAll(mediaFilePath, 0644); err != nil {
		return "", err
	}
	newFile, err := os.Create(filepath.Join(mediaFilePath, filename))
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(newFile, file); err != nil {
		return "", err
	}
	if l.mediaURL != "" {
		return fmt.Sprintf("%s/%s", l.mediaURL, filename), nil
	}
	return fmt.Sprintf("/m/%s", filename), nil
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
	if f.address == "" || f.user == "" || f.password == "" {
		return "", errors.New("missing FTP config")
	}
	c, err := ftp.Dial(f.address, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		return "", err
	}
	defer func() {
		_ = c.Quit()
	}()
	if err = c.Login(f.user, f.password); err != nil {
		return "", err
	}
	if err = c.Stor(filename, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", f.mediaURL, filename), nil
}
