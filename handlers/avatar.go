package handlers

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image"
	"net/http"
	"strconv"
	"strings"

	_ "image/jpeg"
	_ "image/png"

	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/disintegration/imaging"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/renstrom/shortuuid"
	"github.com/satori/go.uuid"
)

type avatarHandler struct {
	db *db.DB
}

func NewAvatarHandler(db *db.DB) *avatarHandler {
	return &avatarHandler{db: db}
}

func (h *avatarHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	memberuid := vars["memberuid"]

	sizeString := r.FormValue("s")
	var size int
	if sizeString != "" {
		var err error
		size, err = strconv.Atoi(sizeString)
		if err != nil {
			log.Errorf("err: %v", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if size > util.AvatarSize {
			err := errors.Errorf("size %d too big", size)
			log.Errorf("err: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	}

	var id uuid.UUID
	var err error
	id, err = shortuuid.DefaultEncoder.Decode(string(memberuid))
	if err != nil {
		id, err = uuid.FromString(string(memberuid))
		if err != nil {
			log.Errorf("err: %v", err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}
	}
	log.Debugf("uuid: %s", id)
	memberid := util.NewFromUUID(id)

	tx, err := h.db.NewTx()
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	avatar, err := readDBService.MemberAvatar(ctx, readDBService.CurTimeLine(ctx).Number(), memberid)
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if avatar == nil {
		http.NotFound(w, r)
		return
	}

	if size > 0 {
		c, _, err := image.DecodeConfig(bytes.NewReader(avatar.Image))
		if err != nil {
			log.Errorf("err: %v", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if c.Width != size {
			m, _, err := image.Decode(bytes.NewReader(avatar.Image))
			if err != nil {
				log.Errorf("err: %v", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			m = imaging.Resize(m, size, size, imaging.Lanczos)
			buf := &bytes.Buffer{}
			if err := imaging.Encode(buf, m, imaging.PNG); err != nil {
				log.Errorf("err: %v", err)
				http.Error(w, "", http.StatusInternalServerError)
				return
			}
			avatar.Image = buf.Bytes()
		}
	}

	etag := fmt.Sprintf("%x", md5.Sum(avatar.Image))
	w.Header().Set("Etag", etag)

	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Write(avatar.Image)
}
