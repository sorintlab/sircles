package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sorintlab/sircles/auth"
	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/coreos/go-oidc"
	jwt "github.com/dgrijalva/jwt-go"
	jwtrequest "github.com/dgrijalva/jwt-go/request"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

type TokenSigningData struct {
	Duration   uint
	Method     jwt.SigningMethod
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	Key        []byte
}

type loginRequest struct {
	Username string
	Password string
}
type loginResponse struct {
	Token string `json:"token"`
}

type oidAuthURLResponse struct {
	URL string `json:"url"`
}

func generateToken(sd *TokenSigningData, userid string) (string, error) {
	token := jwt.NewWithClaims(sd.Method, jwt.MapClaims{
		"sub": userid,
		"exp": time.Now().Add(time.Duration(sd.Duration) * time.Second).Unix(),
	})

	var key interface{}
	switch sd.Method {
	case jwt.SigningMethodRS256:
		key = sd.PrivateKey
	case jwt.SigningMethodHS256:
		key = sd.Key
	default:
		errors.Errorf("unsupported signing method %q", sd.Method.Alg())
	}
	// Sign and get the complete encoded token as a string
	return token.SignedString(key)
}

type loginHandler struct {
	config           *config.Config
	db               *db.DB
	authenticator    auth.Authenticator
	memberProvider   auth.MemberProvider
	tokenSigningData *TokenSigningData
}

func NewLoginHandler(config *config.Config, db *db.DB, authenticator auth.Authenticator, memberProvider auth.MemberProvider, tokenSigningData *TokenSigningData) *loginHandler {
	return &loginHandler{
		config:           config,
		db:               db,
		authenticator:    authenticator,
		memberProvider:   memberProvider,
		tokenSigningData: tokenSigningData,
	}
}

func doAuth(ctx context.Context, authenticator auth.Authenticator, loginName, password, oidcCode string) (string, *oidc.IDToken, error) {
	var (
		matchUID string
		err      error
		idToken  *oidc.IDToken
	)

	switch authenticator := authenticator.(type) {
	case auth.LoginAuthenticator:
		matchUID, err = authenticator.Login(ctx, loginName, password)
		if err != nil {
			return "", nil, err
		}
	case auth.CallbackAuthenticator:
		matchUID, idToken, err = authenticator.HandleCallback(ctx, oidcCode)
		if err != nil {
			return "", nil, err
		}
	default:
		return "", nil, errors.Errorf("unknown authenticator: %v", authenticator)
	}
	return matchUID, idToken, nil
}

func (h *loginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	loginName := r.Form.Get("login")
	password := r.Form.Get("password")
	oidcCode := r.Form.Get("code")

	matchUID, idToken, err := doAuth(ctx, h.authenticator, loginName, password, oidcCode)
	if err != nil {
		log.Errorf("auth err: %+v", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	tx, err := h.db.NewTx()
	if err != nil {
		log.Errorf("err: %+v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		log.Errorf("err: %+v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	commandService := command.NewCommandService(tx, readDB, nil, h.memberProvider != nil)

	// find a matching member using the matchUID reported by the authenticator
	member, err := auth.FindMatchingMember(ctx, readDB, matchUID)
	if err != nil {
		tx.Rollback()
		log.Errorf("auth err: %v", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	// if a memberProvider is defined, get memberinfos from it
	if h.memberProvider != nil {
		memberInfo, err := auth.GetMemberInfo(ctx, h.authenticator, h.memberProvider, loginName, idToken)
		if err != nil {
			log.Errorf("failed to retrieve member info: %v", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		log.Debugf("memberInfo: %#+v", memberInfo)
	}

	// if there isn't a local member for the provided matchUID and no
	// memberprovider is configured don't accept the logged in user
	if member == nil && h.memberProvider == nil {
		log.Errorf("auth err: member with matchUID %q doesn't exists", matchUID)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}

	// if there isn't a local member for the provided matchUID try to import it
	// from the memberProvider
	//
	// TODO(sgotti) also handle updating local member data with the one
	// provided by the member provider
	if member == nil && h.memberProvider != nil {
		memberInfo, err := auth.GetMemberInfo(ctx, h.authenticator, h.memberProvider, loginName, idToken)
		if err != nil {
			log.Errorf("failed to retrieve member info: %v", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		if matchUID != memberInfo.MatchUID {
			log.Errorf("authenticator reported matchUID: %q different from member provider reported matchUID: %q", matchUID, memberInfo.MatchUID)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		log.Debugf("memberInfo: %#+v", memberInfo)
		c := &change.CreateMemberChange{
			IsAdmin:  false,
			MatchUID: memberInfo.MatchUID,
			UserName: memberInfo.UserName,
			FullName: memberInfo.FullName,
			Email:    memberInfo.Email,
		}
		if _, _, err := commandService.CreateMemberInternal(ctx, c, false, false); err != nil {
			log.Errorf("failed to create member: %v", err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		member, err = auth.FindMatchingMember(ctx, readDB, memberInfo.MatchUID)
		if err != nil {
			tx.Rollback()
			log.Errorf("auth err: %v", err)
			http.Error(w, "authentication failed", http.StatusUnauthorized)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	tokenString, err := generateToken(h.tokenSigningData, member.ID.String())
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	lres := loginResponse{tokenString}
	lresj, err := json.Marshal(lres)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(lresj)
}

type oidcAuthURLHandler struct {
	authenticator auth.Authenticator
}

func NewOIDCAuthURLHandler(authenticator auth.Authenticator) *oidcAuthURLHandler {
	return &oidcAuthURLHandler{
		authenticator: authenticator,
	}
}

func (h *oidcAuthURLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	state := r.Form.Get("state")

	var authURL string
	switch authenticator := h.authenticator.(type) {
	default:
		log.Errorf("only oidc authenticator is supported")
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	case auth.CallbackAuthenticator:
		var err error
		authURL, err = authenticator.AuthURL(ctx, state)
		if err != nil {
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	}

	res := oidAuthURLResponse{authURL}
	resj, err := json.Marshal(res)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resj)
}

type refreshTokenHandler struct {
	tokenSigningData *TokenSigningData
}

func NewRefreshTokenHandler(tokenSigningData *TokenSigningData) *refreshTokenHandler {
	return &refreshTokenHandler{
		tokenSigningData: tokenSigningData,
	}
}

func (h *refreshTokenHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// use the provided token username for generating a new token
	userid := r.Context().Value("userid").(string)
	tokenString, err := generateToken(h.tokenSigningData, userid)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	log.Debugf("tokenString: %s\n", tokenString)

	lres := loginResponse{tokenString}
	lresj, err := json.Marshal(lres)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(lresj)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

type AuthHandler struct {
	db               *db.DB
	tokenSigningData *TokenSigningData
	next             http.Handler
}

func NewAuthHandler(db *db.DB, sd *TokenSigningData) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return &AuthHandler{
			db:               db,
			tokenSigningData: sd,
			next:             h,
		}
	}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token, err := jwtrequest.ParseFromRequest(r, jwtrequest.AuthorizationHeaderExtractor, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg
		sd := h.tokenSigningData
		if token.Method != sd.Method {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		var key interface{}
		switch sd.Method {
		case jwt.SigningMethodRS256:
			key = sd.PrivateKey
		case jwt.SigningMethodHS256:
			key = sd.Key
		default:
			return nil, errors.Errorf("unsupported signing method %q", sd.Method.Alg())
		}
		return key, nil
	})
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	if !token.Valid {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	// Set username in the request context
	claims := token.Claims.(jwt.MapClaims)

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

	userIDString, ok := claims["sub"].(string)
	if !ok {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}
	userID, err := uuid.FromString(userIDString)
	if err != nil {
		log.Errorf("err: %v", err)
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	member, err := readDB.MemberInternal(readDB.CurTimeLine().SequenceNumber, util.NewFromUUID(userID))
	if err != nil {
		tx.Rollback()
		log.Errorf("auth err: %v", err)
		// mask reported error
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	if member == nil {
		tx.Rollback()
		log.Errorf("member with id %s doesn't exist", userID)
		// mask reported error
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	ctx := r.Context()
	ctx = context.WithValue(ctx, "userid", userIDString)
	log.Debugf("userid: %s", ctx.Value("userid"))
	h.next.ServeHTTP(w, r.WithContext(ctx))
}
