package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_compress(t *testing.T) {
	fakeFileContent := "Test"
	fakeFileName := filepath.Join(t.TempDir(), "test.jpg")
	err := os.WriteFile(fakeFileName, []byte(fakeFileContent), 0777)
	require.Nil(t, err)
	fakeFile, err := os.Open(fakeFileName)
	require.Nil(t, err)
	fakeSha256, err := getSHA256(fakeFile)
	require.Nil(t, err)

	var uf fileUploadFunc = func(filename string, f io.Reader) (location string, err error) {
		return "https://example.com/" + filename, nil
	}

	t.Run("Cloudflare", func(t *testing.T) {
		fakeClient := getFakeHTTPClient()
		fakeClient.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "https://www.cloudflare.com/cdn-cgi/image/f=jpeg,q=75,metadata=none,fit=scale-down,w=2000,h=3000/https://example.com/original.jpg", r.URL.String())

			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(fakeFileContent))
		}))

		cf := &cloudflare{}
		res, err := cf.compress("https://example.com/original.jpg", uf, fakeClient)

		assert.Nil(t, err)
		assert.Equal(t, "https://example.com/"+fakeSha256+".jpeg", res)
	})

	t.Run("Shortpixel", func(t *testing.T) {
		fakeClient := getFakeHTTPClient()
		fakeClient.setHandler(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "https://api.shortpixel.com/v2/reducer-sync.php", r.URL.String())

			requestBody, _ := io.ReadAll(r.Body)
			defer r.Body.Close()

			var requestJson map[string]interface{}
			err = json.Unmarshal(requestBody, &requestJson)
			require.Nil(t, err)
			require.NotNil(t, requestJson)

			assert.Equal(t, "testkey", requestJson["key"])
			assert.Equal(t, "https://example.com/original.jpg", requestJson["url"])

			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(fakeFileContent))
		}))

		cf := &shortpixel{"testkey"}
		res, err := cf.compress("https://example.com/original.jpg", uf, fakeClient)

		assert.Nil(t, err)
		assert.Equal(t, "https://example.com/"+fakeSha256+".jpg", res)
	})

}
