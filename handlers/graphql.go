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
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/search"

	"github.com/neelance/graphql-go"
)

type graphqlHandler struct {
	config         *config.Config
	db             *db.DB
	searchEngine   *search.SearchEngine
	schema         *graphql.Schema
	memberProvider auth.MemberProvider
}

func NewGraphQLHandler(config *config.Config, db *db.DB, searchEngine *search.SearchEngine, schema *graphql.Schema, memberProvider auth.MemberProvider) *graphqlHandler {
	return &graphqlHandler{
		config:         config,
		db:             db,
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
					log.Errorf("err: %v", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}
			} else {
				// handle files. Now only one image is handled
				// TODO(sgotti) instead of keeping the image in memory save it to a temporary file?
				image, err = ioutil.ReadAll(part)
				if err != nil {
					log.Errorf("err: %v", err)
					http.Error(w, "", http.StatusInternalServerError)
					return
				}
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			log.Errorf("err: %v", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
	}

	tx, err := h.db.NewTx()
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if h.config.AdminMember != "" {
		readDB.SetForceAdminMemberUserName(h.config.AdminMember)
	}
	commandService := command.NewCommandService(tx, readDB, nil, nil, h.memberProvider != nil)

	ctx := r.Context()
	ctx = context.WithValue(ctx, "service", readDB)
	ctx = context.WithValue(ctx, "commandservice", commandService)
	ctx = context.WithValue(ctx, "memberprovider", h.memberProvider)
	ctx = context.WithValue(ctx, "searchEngine", h.searchEngine)
	ctx = context.WithValue(ctx, "image", image)

	log.Debugf("graphql exec")
	response := h.schema.Exec(ctx, params.Query, params.OperationName, params.Variables)

	// NOTE(sgotti)
	// The first iteration used a simpler model where every service call done
	// inside the graphql Schema.Exec used its own managed transaction. Thanks
	// the db immutable model the full result was consistent because we asked
	// the same timeline to every service function. So there's no need to wrap
	// the full graphql call inside an unique db transaction
	// This version instead uses an unique db transaction with a serializable
	// isolation level.
	// Having only one transaction is faster and can be useful as an example for
	// other implementation that requires an unique transaction to get
	// consistent results
	// As an addition a query with multiple mutations will execute them inside
	// the same transaction so they will be committed only if all the mutations
	// have success.

	// To have one transaction per graphql call the unique solution is to create
	// a service instance per call before calling schema.Exec.
	// Since neelance/graphql-go accepts a single instance at schema creation
	// time the unique thing we can do is pass this service instance using
	// the context (I'm not sure this a good way to use the context).

	// TODO as an addition, we could parse the graphql query to know if the
	// query contains only queries, only mutation or both we could create
	// different kind of transactions (a readonly one for query only and a
	// readwrite for mutations).
	if len(response.Errors) > 0 {
		log.Errorf("graphql errors: %v", response.Errors)
		tx.Rollback()
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	// TODO(sgotti) hanle postgresql serialization error and retry mutations
	if err := tx.Commit(); err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}
