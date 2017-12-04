package handlers

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/sorintlab/sircles/auth"
	"github.com/sorintlab/sircles/command"
	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	ln "github.com/sorintlab/sircles/listennotify"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/search"

	"github.com/neelance/graphql-go"
)

type graphqlHandler struct {
	config         *config.Config
	dataDir        string
	readDB         *db.DB
	readDBListener readdb.ReadDBListener
	es             *eventstore.EventStore
	lnf            ln.ListenerFactory
	searchEngine   *search.SearchEngine
	schema         *graphql.Schema
	memberProvider auth.MemberProvider
}

func NewGraphQLHandler(config *config.Config, dataDir string, readDB *db.DB, readDBListener readdb.ReadDBListener, es *eventstore.EventStore, lnf ln.ListenerFactory, searchEngine *search.SearchEngine, schema *graphql.Schema, memberProvider auth.MemberProvider) *graphqlHandler {
	return &graphqlHandler{
		config:         config,
		dataDir:        dataDir,
		readDB:         readDB,
		readDBListener: readDBListener,
		es:             es,
		lnf:            lnf,
		searchEngine:   searchEngine,
		schema:         schema,
		memberProvider: memberProvider,
	}
}

func (h *graphqlHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	log.Debugf("content-type: %s", r.Header.Get("content-type"))

	isMultipart := true
	reader, err := r.MultipartReader()
	if err != nil {
		if err == http.ErrNotMultipart {
			isMultipart = false
		} else {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	var image []byte
	if isMultipart {
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}

			log.Debugf("part: %s %s", part.FileName(), part.FormName())

			if part.FormName() == "operations" {
				if err := json.NewDecoder(part).Decode(&params); err != nil {
					log.Errorf("err: %+v", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}
			} else {
				// handle files. Now only one image is handled
				// TODO(sgotti) instead of keeping the image in memory save it to a temporary file?
				image, err = ioutil.ReadAll(part)
				if err != nil {
					log.Errorf("err: %+v", err)
					http.Error(w, "", http.StatusInternalServerError)
					return
				}
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			log.Errorf("err: %+v", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
	}

	commandService := command.NewCommandService(h.dataDir, h.readDB, h.es, nil, h.lnf, h.memberProvider != nil)

	// NOTE(sgotti) only for performance reasons we want to query the readdb
	// within a single transaction. Since the graphql library calls various
	// methods we don't have a final point inside the graphql schema exec func where
	// to commit/rollback the transaction. So the hack is to create here an
	// unstarted transaction, start it inside the schema when needed and
	// commit/rollback it here if it has been started.
	// For graphql mutations we execute the command, wait for the readdb to have
	// applied the events and then query it to return the response.

	utx := h.readDB.NewUnstartedTx()

	ctx := r.Context()
	ctx = context.WithValue(ctx, "utx", utx)
	ctx = context.WithValue(ctx, "config", h.config)
	ctx = context.WithValue(ctx, "readdblistener", h.readDBListener)
	ctx = context.WithValue(ctx, "commandservice", commandService)
	ctx = context.WithValue(ctx, "memberprovider", h.memberProvider)
	ctx = context.WithValue(ctx, "searchEngine", h.searchEngine)
	ctx = context.WithValue(ctx, "image", image)

	log.Debugf("graphql exec")
	response := h.schema.Exec(ctx, params.Query, params.OperationName, params.Variables)

	if len(response.Errors) > 0 {
		utx.Rollback()
		log.Errorf("graphql errors: %v", response.Errors)
		for _, err := range response.Errors {
			log.Errorf("err: %+v", err.ResolverError)
		}
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if err := utx.Commit(); err != nil {
		log.Errorf("err: %+v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Errorf("err: %+v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}
