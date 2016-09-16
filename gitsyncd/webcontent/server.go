package webcontent

import (
	"bytes"
	"encoding/hex"
	"fmt"
	log "github.com/ngmoco/timber"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

type mapHandler struct {
	templatePaths map[string]*template.Template
	staticPaths   map[string][]byte
}

func NewMapHandler(paths map[string]string) (handler http.Handler, err error) {
	h := mapHandler{
		templatePaths: make(map[string]*template.Template),
		staticPaths:   make(map[string][]byte)}

	for path, data := range paths {
		log.Debug("Parsing in web asset %s", path)
		dataBytes, err := hex.DecodeString(data)
		if err != nil {
			return nil, err
		}

		switch {
		case strings.HasPrefix(path, "templates/"):
			log.Debug("Parsing in templated web asset glob %s", path)
			t, err := template.New(path).Parse(string(dataBytes))
			if err != nil {
				return nil, fmt.Errorf("Error parsing %s: %s", path, err)
			}
			log.Debug("Adding templated asset %s to lookup", path)
			h.templatePaths[path] = t

		default:
			log.Debug("Adding static asset %s to lookup", path)
			h.staticPaths[path] = dataBytes
		}
	}

	return h, nil
}

func (m mapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		upath = r.URL.Path
		ctype = mime.TypeByExtension(filepath.Ext(upath))
	)

	w.Header().Set("Content-Type", ctype)

	log.Debug("Handling %s", upath)
	if t, found := m.templatePaths[upath]; found {
		log.Debug("Found data for %s of type %s", upath, ctype)
		if err := t.Execute(w, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	} else if t, found := m.staticPaths[upath]; found {
		log.Debug("Found data for %s of type %s", upath, ctype)
		io.Copy(w, bytes.NewBuffer(t))
	} else {
		log.Debug("Found no data for %s", upath)
		w.WriteHeader(404)
	}
}
