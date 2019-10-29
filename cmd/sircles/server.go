package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	graphqlapi "github.com/sorintlab/sircles/api/graphql"
	"github.com/sorintlab/sircles/auth"
	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventhandler"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/handlers"
	ln "github.com/sorintlab/sircles/listennotify"
	"github.com/sorintlab/sircles/lock"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/search"

	jwt "github.com/dgrijalva/jwt-go"
	ghandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/neelance/graphql-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
)

func strAddr(s string) *string {
	return &s
}

var configFile, dumpFile string

var log = slog.S()

var rootCmd = &cobra.Command{
	Use: "sircles",
}

var serveCmd = &cobra.Command{
	Use: "serve",
	Run: func(cmd *cobra.Command, args []string) {
		if err := serve(cmd, args); err != nil {
			log.Errorf("err: %+v", err)
			os.Exit(-1)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "path to configuration file")

	rootCmd.AddCommand(serveCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(-1)
	}
}

func serve(cmd *cobra.Command, args []string) error {
	if configFile == "" {
		return errors.New("you should provide a config file path (-c option)")
	}

	c, err := config.Parse(configFile)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("error parsing configuration file %s", configFile))
	}

	if c.Debug {
		slog.SetLevel(zapcore.DebugLevel)
	}

	if c.Web.HTTP == "" && c.Web.HTTPS == "" {
		return errors.Errorf("at least one between a http or https address must be defined")
	}

	if c.Web.HTTPS != "" {
		if c.Web.TLSKey == "" {
			return errors.Errorf("no tls key specified")
		}
		if c.Web.TLSCert == "" {
			return errors.Errorf("no tls cert specified")
		}
	}

	if c.ReadDB.Type == "" {
		return errors.New("no read db type specified")
	}

	if c.EventStore.Type == "" {
		return errors.New("no eventstore type specified")
	}
	if c.EventStore.Type != "sql" {
		return errors.Errorf("unknown eventstore type: %q", c.EventStore.Type)
	}
	if c.EventStore.DB.Type == "" {
		return errors.New("no eventstore db type specified")
	}

	switch c.ReadDB.Type {
	case db.Postgres:
	case db.Sqlite3:
	default:
		return errors.Errorf("unsupported read db type: %s", c.ReadDB.Type)
	}

	switch c.EventStore.DB.Type {
	case db.Postgres:
	case db.Sqlite3:
	default:
		return errors.Errorf("unsupported eventstore db type: %s", c.EventStore.DB.Type)
	}

	readDBLnType := getLNtype(&c.ReadDB)
	esLnType := getLNtype(&c.EventStore.DB)

	readDBLf, readDBNf, err := getListenerNotifierFactories(readDBLnType, &c.ReadDB)
	if err != nil {
		return err
	}
	esLf, esNf, err := getListenerNotifierFactories(esLnType, &c.EventStore.DB)
	if err != nil {
		return err
	}

	tokenSigningData := &handlers.TokenSigningData{Duration: c.TokenSigning.Duration}
	switch c.TokenSigning.Method {
	case "hmac":
		tokenSigningData.Method = jwt.SigningMethodHS256
		if c.TokenSigning.Key == "" {
			return errors.Errorf("empty token signing key for hmac method")
		}
		tokenSigningData.Key = []byte(c.TokenSigning.Key)
	case "rsa":
		if c.TokenSigning.PrivateKeyPath == "" {
			return errors.Errorf("token signing private key file for rsa method not defined")
		}
		if c.TokenSigning.PublicKeyPath == "" {
			return errors.Errorf("token signing public key file for rsa method not defined")
		}

		tokenSigningData.Method = jwt.SigningMethodRS256
		privateKeyData, err := ioutil.ReadFile(c.TokenSigning.PrivateKeyPath)
		if err != nil {
			return errors.Wrapf(err, "error reading token signing private key")
		}
		tokenSigningData.PrivateKey, err = jwt.ParseRSAPrivateKeyFromPEM(privateKeyData)
		if err != nil {
			return errors.Wrapf(err, "error parsing token signing private key")
		}
		publicKeyData, err := ioutil.ReadFile(c.TokenSigning.PublicKeyPath)
		if err != nil {
			return errors.Wrapf(err, "error reading token signing public key")
		}
		tokenSigningData.PublicKey, err = jwt.ParseRSAPublicKeyFromPEM(publicKeyData)
		if err != nil {
			return errors.Wrapf(err, "error parsing token signing public key")
		}
	case "":
		return errors.Errorf("missing token signing method")
	default:
		return errors.Errorf("unknown token signing method: %q", c.TokenSigning.Method)
	}

	readDB, err := db.NewDB(c.ReadDB.Type, c.ReadDB.ConnString)
	if err != nil {
		return err
	}

	// Populate/migrate readdb
	if err := readDB.Migrate("readdb", readdb.Migrations); err != nil {
		return err
	}

	esDB, err := db.NewDB(c.EventStore.DB.Type, c.EventStore.DB.ConnString)
	if err != nil {
		return err
	}

	// Populate/migrate esdb
	if err := esDB.Migrate("eventstore", eventstore.Migrations); err != nil {
		return err
	}

	lkf, err := getLockFactory(&c.EventStore.DB, esDB)
	if err != nil {
		return err
	}

	var authenticator auth.Authenticator

	switch c.Authentication.Type {
	case "local":
		authConf := c.Authentication.Config.(*config.LocalAuthConfig)
		authenticator = auth.NewLocalAuthenticator(authConf, readDB)
	case "ldap":
		authConf := c.Authentication.Config.(*config.LDAPAuthConfig)
		authenticator, err = auth.NewLDAPAuthenticator(authConf)
		if err != nil {
			return err
		}
	case "oidc":
		authConf := c.Authentication.Config.(*config.OIDCAuthConfig)
		authenticator, err = auth.NewOIDCAuthenticator(authConf)
		if err != nil {
			return err
		}
	}

	var memberProvider auth.MemberProvider

	switch c.MemberProvider.Type {
	case "ldap":
		mpConf := c.MemberProvider.Config.(*config.LDAPMemberProviderConfig)
		memberProvider, err = auth.NewLDAPMemberProvider(mpConf)
		if err != nil {
			return err
		}
	case "oidc":
		mpConf := c.MemberProvider.Config.(*config.OIDCMemberProviderConfig)
		memberProvider, err = auth.NewOIDCMemberProvider(mpConf)
		if err != nil {
			return err
		}
	}

	readDBListener := readdb.NewDBListener(readDB, readDBLf)
	es := eventstore.NewEventStore(esDB, esNf)

	resolver := graphqlapi.NewResolver()
	// Since we are using dataloaders to avoid N+1 queries problem we want to
	// parallelize resolvers executions
	s, err := graphql.ParseSchema(graphqlapi.Schema, resolver, graphql.MaxParallelism(1000))
	if err != nil {
		return err
	}

	searchEngine := search.NewSearchEngine(readDB, es, c.Index.Path)

	// noop coors handler
	corsHandler := func(h http.Handler) http.Handler {
		return h
	}

	if len(c.Web.AllowedOrigins) > 0 {
		corsAllowedHeadersOptions := ghandlers.AllowedHeaders([]string{"Accept", "Accept-Encoding", "Authorization", "Content-Length", "Content-Type", "X-CSRF-Token", "Authorization"})
		corsAllowedOriginsOptions := ghandlers.AllowedOrigins(c.Web.AllowedOrigins)
		corsHandler = ghandlers.CORS(corsAllowedHeadersOptions, corsAllowedOriginsOptions)
	}

	// For the moment, create a different temporary dataDir for every
	// new process execution.
	// This will trigger the rebuild from scratch, when loaded, of the
	// aggregates/handlers snapshot db (rolestree aggregate, deletedroletension
	// handler).
	dataDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dataDir)

	loginHandler := handlers.NewLoginHandler(c, dataDir, readDB, es, esLf, authenticator, memberProvider, tokenSigningData)
	refreshTokenHandler := handlers.NewRefreshTokenHandler(tokenSigningData)
	oidcAuthURLHandler := handlers.NewOIDCAuthURLHandler(authenticator)
	graphqlHandler := handlers.NewGraphQLHandler(c, dataDir, readDB, readDBListener, es, esLf, searchEngine, s, memberProvider)
	authHandler := handlers.NewAuthHandler(readDB, tokenSigningData)

	router := mux.NewRouter()
	apirouter := router.PathPrefix("/api/").Subrouter()
	apirouter.Handle("/auth/login", loginHandler).Methods("POST")
	apirouter.Handle("/auth/oidcauthurl", oidcAuthURLHandler).Methods("POST")
	apirouter.Handle("/auth/refresh", authHandler(refreshTokenHandler)).Methods("POST")
	apirouter.Handle("/auth/logout", authHandler(http.HandlerFunc(handlers.LogoutHandler))).Methods("POST")
	apirouter.Handle("/graphql", authHandler(graphqlHandler))
	// TODO(sgotti) since we are providing avatars for browser displaying we can't
	// protect them because the browser img src cannot send the auth token. If
	// protecting the avatar becomes important there's the need to find a way on
	// how to do this.
	apirouter.Handle("/avatar/{memberuid}", handlers.NewAvatarHandler(readDB))

	// Setup serving of bundled webapp from the root path, registered after api
	// handlers or it'll match all the requested paths
	router.PathPrefix("/").HandlerFunc(handlers.NewWebBundleHandlerFunc(c))

	maxBytesHandler := handlers.NewMaxBytesHandler(router, 1024*1024)

	// mainrouter is the main router that wraps all the other routers with the
	// corsHandler and the maxBytesHandler
	mainrouter := mux.NewRouter()
	mainrouter.PathPrefix("/").Handler(corsHandler(maxBytesHandler))

	listenErrChan := make(chan error, 3)
	if c.Web.HTTP != "" {
		log.Infof("http listening on %s", c.Web.HTTP)
		go func() {
			err := http.ListenAndServe(c.Web.HTTP, mainrouter)
			listenErrChan <- errors.Wrapf(err, "listening on %s failed", c.Web.HTTP)
		}()
	}
	if c.Web.HTTPS != "" {
		log.Infof("https listening on %s", c.Web.HTTPS)
		go func() {
			err := http.ListenAndServeTLS(c.Web.HTTPS, c.Web.TLSCert, c.Web.TLSKey, mainrouter)
			listenErrChan <- errors.Wrapf(err, "listening on %s failed", c.Web.HTTPS)
		}()
	}

	stop := make(chan struct{})
	endChs := []chan struct{}{}

	readDBh := readdb.NewDBEventHandler(readDB, es, readDBNf)
	mrh := eventhandler.NewMemberRequestHandler(es, &common.DefaultUidGenerator{})
	drth, err := eventhandler.NewDeletedRoleTensionHandler(dataDir, es, &common.DefaultUidGenerator{})
	if err != nil {
		return err
	}

	for _, h := range []eventhandler.EventHandler{readDBh, mrh, drth} {
		endCh, err := eventhandler.RunEventHandler(h, stop, esLf, lkf)
		if err != nil {
			return err
		}
		endChs = append(endChs, endCh)
	}

	if err := initializeSircles(dataDir, readDB, es, readDBLf, esLf, c.CreateInitialAdmin); err != nil {
		return err
	}

	return <-listenErrChan
}

func initializeSircles(dataDir string, readDB *db.DB, es *eventstore.EventStore, readDBLf, esLf ln.ListenerFactory, createInitialAdmin bool) error {
	events, err := es.GetAllEvents(0, 1)
	if err != nil {
		return err
	}
	ctx := context.Background()
	// initialize only if the eventstore is empty
	if len(events) > 0 {
		return nil
	}

	commandService := command.NewCommandService(dataDir, readDB, es, nil, esLf, false)

	readDBListener := readdb.NewDBListener(readDB, readDBLf)

	_, groupID, err := commandService.SetupRootRole()
	if err != nil {
		return err
	}

	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		return err
	}

	if createInitialAdmin {
		c := &change.CreateMemberChange{
			IsAdmin:  true,
			UserName: "admin",
			FullName: "Admin",
			Email:    "admin@example.com",
			Password: "password",
		}

		if _, _, err := commandService.CreateMemberInternal(ctx, c, false, false); err != nil {
			return err
		}
	}

	return nil
}

func getLNtype(dbConfig *config.DB) ln.Type {
	var lnType ln.Type
	switch dbConfig.Type {
	case db.Postgres:
		lnType = ln.Postgres
	case db.Sqlite3:
		lnType = ln.Local
	default:
		panic(errors.Errorf("unsupported db type: %s", dbConfig.Type))
	}
	return lnType
}

func getListenerNotifierFactories(lnType ln.Type, dbConfig *config.DB) (ln.ListenerFactory, ln.NotifierFactory, error) {
	var lf ln.ListenerFactory
	var nf ln.NotifierFactory
	switch lnType {
	case ln.Local:
		localln := ln.NewLocalListenNotify()
		lf = ln.NewLocalListenerFactory(localln)
		nf = ln.NewLocalNotifierFactory(localln)
	case ln.Postgres:
		lf = ln.NewPGListenerFactory(dbConfig.ConnString)
		nf = ln.NewPGNotifierFactory()
	default:
		return nil, nil, errors.Errorf("unknown listener type: %q", lnType)
	}
	return lf, nf, nil
}

func getLockFactory(dbConfig *config.DB, d *db.DB) (lock.LockFactory, error) {
	var lkf lock.LockFactory
	switch dbConfig.Type {
	case db.Postgres:
		lkf = lock.NewPGLockFactory(common.EventHandlersLockSpace, d)
	case db.Sqlite3:
		locallocks := lock.NewLocalLocks()
		lkf = lock.NewLocalLockFactory(locallocks)
	default:
		return nil, errors.Errorf("unknown db type: %q", dbConfig.Type)
	}
	return lkf, nil
}
