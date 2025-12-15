package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"goftp.io/server/v2"
	"goftp.io/server/v2/driver/file"
)

func Test_localMediaStorage_location(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		storage := &localMediaStorage{path: t.TempDir()}
		assert.Equal(t, "/m/asset.png", storage.location("asset.png"))
	})

	t.Run("customMediaURL", func(t *testing.T) {
		storage := &localMediaStorage{
			path:     t.TempDir(),
			mediaURL: "https://cdn.example.com/media",
		}
		assert.Equal(t, "https://cdn.example.com/media/asset.png", storage.location("asset.png"))
	})
}

func Test_localMediaStorage_lifecycle(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{
		path:     storagePath,
		mediaURL: "https://cdn.example.com",
	}

	const fileName = "hello.txt"
	const fileContent = "This is a test"

	loc, err := storage.save(fileName, strings.NewReader(fileContent))
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/hello.txt", loc)

	data, err := os.ReadFile(filepath.Join(storagePath, fileName))
	require.NoError(t, err)
	assert.Equal(t, fileContent, string(data))

	files, err := storage.files()
	require.NoError(t, err)
	if assert.Len(t, files, 1) {
		assert.Equal(t, fileName, files[0].Name)
		assert.Equal(t, loc, files[0].Location)
		assert.Equal(t, int64(len(fileContent)), files[0].Size)
	}

	require.NoError(t, storage.delete(fileName))
	_, err = os.Stat(filepath.Join(storagePath, fileName))
	assert.ErrorIs(t, err, os.ErrNotExist)

	files, err = storage.files()
	require.NoError(t, err)
	assert.Empty(t, files)
}

func Test_localMediaStorage_filesIgnoresDirectories(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{
		path:     storagePath,
		mediaURL: "https://cdn.example.com",
	}

	require.NoError(t, os.Mkdir(filepath.Join(storagePath, "nested"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(storagePath, "one.txt"), []byte("abc"), 0o644))

	files, err := storage.files()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "one.txt", files[0].Name)
	assert.Equal(t, storage.location("one.txt"), files[0].Location)
	assert.Equal(t, int64(3), files[0].Size)
}

func Test_localMediaStorage_deleteMissingFile(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), "media")
	storage := &localMediaStorage{path: basePath}

	err := storage.delete("missing.txt")
	assert.ErrorIs(t, err, os.ErrNotExist)

	info, statErr := os.Stat(basePath)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func Test_goBlog_localMediaStorageIntegration(t *testing.T) {
	storagePath := t.TempDir()
	storage := &localMediaStorage{path: storagePath}
	app := newAppWithStorage(t, storage)

	assert.True(t, app.mediaStorageEnabled())

	const fileName = "media.txt"
	const fileContent = "integrated content"
	loc, err := app.saveMediaFile(fileName, strings.NewReader(fileContent))
	require.NoError(t, err)
	assert.Equal(t, app.cfg.Server.getFullAddress(storage.location(fileName)), loc)

	data, err := os.ReadFile(filepath.Join(storagePath, fileName))
	require.NoError(t, err)
	assert.Equal(t, fileContent, string(data))

	files, err := app.mediaFiles()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, fileName, files[0].Name)
	assert.Equal(t, storage.location(fileName), files[0].Location)

	assert.Equal(t, storage.location(fileName), app.mediaFileLocation(fileName))

	require.NoError(t, app.deleteMediaFile(fileName))
	files, err = app.mediaFiles()
	require.NoError(t, err)
	assert.Empty(t, files)
	_, err = os.Stat(filepath.Join(storagePath, fileName))
	assert.ErrorIs(t, err, os.ErrNotExist)

	err = app.deleteMediaFile("../escape.txt")
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func Test_goBlog_saveMediaFileWithoutStorage(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.mediaStorageInit.Do(func() {})

	_, err := app.saveMediaFile("file.txt", strings.NewReader("content"))
	assert.ErrorIs(t, err, errNoMediaStorageConfigured)
}

func Test_mediaStorageDisabled(t *testing.T) {
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.mediaStorageInit.Do(func() {})

	assert.False(t, app.mediaStorageEnabled())
	assert.Empty(t, app.mediaFileLocation("unused"))
	_, err := app.mediaFiles()
	require.ErrorIs(t, err, errNoMediaStorageConfigured)
}

func Test_bunnyMediaStorage_httpLifecycle(t *testing.T) {
	var received []string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = append(received, r.Method+" "+r.URL.Path)
		assert.Equal(t, "key", r.Header.Get("AccessKey"))

		switch r.Method {
		case http.MethodPut, http.MethodDelete:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"ObjectName":"file.txt","Length":4,"LastChanged":"2020-01-02T03:04:05Z","IsDirectory":false},{"ObjectName":"dir","Length":0,"LastChanged":"2020-01-02T03:04:05Z","IsDirectory":true}]`))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer ts.Close()

	storage := &bunnyMediaStorage{
		address:    ts.URL + "/",
		apiKey:     "key",
		mediaURL:   "https://media.example",
		httpClient: ts.Client(),
	}

	loc, err := storage.save("file.txt", strings.NewReader("data"))
	require.NoError(t, err)
	assert.Equal(t, "https://media.example/file.txt", loc)

	require.NoError(t, storage.delete("file.txt"))

	files, err := storage.files()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "file.txt", files[0].Name)
	assert.Equal(t, "https://media.example/file.txt", files[0].Location)
	assert.Equal(t, int64(4), files[0].Size)
	assert.True(t, files[0].Time.Equal(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)))

	assert.Equal(t, []string{"PUT /file.txt", "DELETE /file.txt", "GET /"}, received)
}

func Test_ftpMediaStorage_errorsWithoutConfig(t *testing.T) {
	storage := &ftpMediaStorage{}

	_, err := storage.save("file.txt", strings.NewReader("data"))
	assert.Error(t, err)

	assert.Error(t, storage.delete("file.txt"))

	_, err = storage.files()
	assert.Error(t, err)

	_, err = storage.connection()
	assert.Error(t, err)
}

func Test_ftpMediaStorage_integration(t *testing.T) {
	root := t.TempDir()
	address, shutdown := startTestFTPServer(t, root)
	t.Cleanup(shutdown)

	storage := &ftpMediaStorage{
		address:  address,
		user:     "user",
		password: "pass",
		mediaURL: "https://media.example",
	}

	const fname = "sample.txt"
	loc, err := storage.save(fname, strings.NewReader("data"))
	require.NoError(t, err)
	assert.Equal(t, "https://media.example/sample.txt", loc)

	data, err := os.ReadFile(filepath.Join(root, fname))
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))

	files, err := storage.files()
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, fname, files[0].Name)
	assert.Equal(t, "https://media.example/sample.txt", files[0].Location)
	assert.Equal(t, int64(4), files[0].Size)

	require.NoError(t, storage.delete(fname))
	_, err = os.Stat(filepath.Join(root, fname))
	assert.ErrorIs(t, err, os.ErrNotExist)

	files, err = storage.files()
	require.NoError(t, err)
	assert.Empty(t, files)
}

func Test_isValidMediaFilename(t *testing.T) {
	tests := []struct {
		desc  string
		input string
		want  bool
	}{
		{desc: "empty", input: "", want: false},
		{desc: "normal", input: "picture.png", want: true},
		{desc: "parentDirectory", input: "..", want: false},
		{desc: "nested", input: "folder/image.png", want: false},
		{desc: "backslash", input: "bad\\path.png", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			assert.Equal(t, tt.want, isValidMediaFilename(tt.input))
		})
	}
}

func newAppWithStorage(t *testing.T, storage mediaStorage) *goBlog {
	t.Helper()
	app := &goBlog{cfg: createDefaultTestConfig(t)}
	app.mediaStorage = storage
	app.mediaStorageInit.Do(func() {})
	return app
}

func startTestFTPServer(t *testing.T, root string) (string, func()) {
	t.Helper()

	driver, err := file.NewDriver(root)
	require.NoError(t, err)

	opts := &server.Options{
		Name:         "test-ftp",
		Driver:       driver,
		Auth:         &server.SimpleAuth{Name: "user", Password: "pass"},
		Perm:         server.NewSimplePerm("user", "group"),
		Hostname:     "127.0.0.1",
		Port:         freePort(t),
		PassivePorts: "30000-30009",
		Logger:       &server.DiscardLogger{},
	}

	srv, err := server.NewServer(opts)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		_ = srv.ListenAndServe()
		close(done)
	}()

	address := fmt.Sprintf("%s:%d", opts.Hostname, opts.Port)
	require.Eventually(t, func() bool {
		c, err := ftp.Dial(address, ftp.DialWithTimeout(time.Second))
		if err != nil {
			return false
		}
		_ = c.Login("user", "pass")
		_ = c.Quit()
		return true
	}, 5*time.Second, 100*time.Millisecond)

	return address, func() {
		_ = srv.Shutdown()
		<-done
	}
}

func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}
