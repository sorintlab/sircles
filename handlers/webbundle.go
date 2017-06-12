package handlers

import (
	"bytes"
	"net/http"
	"text/template"

	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/webbundle"

	"github.com/elazarl/go-bindata-assetfs"
)

const configTplText = `
const CONFIG = {
  apiBaseUrl: '/api',

  authType: '{{.AuthType}}'
}

window.CONFIG = CONFIG
`

func NewWebBundleHandlerFunc(c *config.Config) func(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	configTpl, err := template.New("config").Parse(configTplText)
	if err != nil {
		panic(err)
	}

	configTplData := struct {
		AuthType string
	}{
		c.Authentication.Type,
	}
	configTpl.Execute(&buf, configTplData)

	config := buf.Bytes()

	return func(w http.ResponseWriter, r *http.Request) {
		// Setup serving of bundled webapp from the root path, registered after api
		// handlers or it'll match all the requested paths
		fileServerHandler := http.FileServer(&assetfs.AssetFS{
			Asset:     webbundle.Asset,
			AssetDir:  webbundle.AssetDir,
			AssetInfo: webbundle.AssetInfo,
		})
		// check if the required file is available in the webapp asset and serve it
		if _, err := webbundle.Asset(r.URL.Path[1:]); err == nil {
			fileServerHandler.ServeHTTP(w, r)
			return
		}
		// config.js is the external webapp config file not provided by the
		// asset and not needed when served from the api server
		if r.URL.Path == "/config.js" {
			_, err := w.Write(config)
			if err != nil {
				http.Error(w, "", http.StatusInternalServerError)
			}
			return
		}
		// Fallback to index.html for every other page. Required for the SPA since
		// on browser reload it'll ask the current app url but we have to
		// provide the index.html
		r.URL.Path = "/"
		fileServerHandler.ServeHTTP(w, r)
	}
}
