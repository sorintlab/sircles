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
	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/handlers"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/search"

	jwt "github.com/dgrijalva/jwt-go"
	ghandlers "github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/neelance/graphql-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
			fmt.Fprintln(os.Stderr, err)
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
		fmt.Println(err)
		os.Exit(-1)
	}
}

func serve(cmd *cobra.Command, args []string) error {
	if configFile == "" {
		return errors.New("you should provide a config file path (-c option)")
	}

	c, err := config.Parse(configFile)
	if err != nil {
		return fmt.Errorf("error parsing configuration file %s: %v", configFile, err)
	}

	if c.Debug {
		slog.SetDebug(true)
	}

	if c.Web.HTTP == "" && c.Web.HTTPS == "" {
		return fmt.Errorf("at least one between a http or https address must be defined")
	}

	if c.Web.HTTPS != "" {
		if c.Web.TLSKey == "" {
			return fmt.Errorf("no tls key specified")
		}
		if c.Web.TLSCert == "" {
			return fmt.Errorf("no tls cert specified")
		}
	}

	if c.DB.Type == "" {
		return errors.New("no db type specified")
	}
	switch c.DB.Type {
	case "postgres":
	case "cockroachdb":
	case "sqlite3":
	default:
		return fmt.Errorf("unsupported db type: %s", c.DB.Type)
	}

	tokenSigningData := &handlers.TokenSigningData{Duration: c.TokenSigning.Duration}
	switch c.TokenSigning.Method {
	case "hmac":
		tokenSigningData.Method = jwt.SigningMethodHS256
		if c.TokenSigning.Key == "" {
			return fmt.Errorf("empty token signing key for hmac method")
		}
		tokenSigningData.Key = []byte(c.TokenSigning.Key)
	case "rsa":
		if c.TokenSigning.PrivateKeyPath == "" {
			return fmt.Errorf("token signing private key file for rsa method not defined")
		}
		if c.TokenSigning.PublicKeyPath == "" {
			return fmt.Errorf("token signing public key file for rsa method not defined")
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
		return fmt.Errorf("missing token signing method")
	default:
		return fmt.Errorf("unknown token signing method: %q", c.TokenSigning.Method)
	}

	db, err := db.NewDB(c.DB.Type, c.DB.ConnString)
	if err != nil {
		return err
	}

	var authenticator auth.Authenticator

	switch c.Authentication.Type {
	case "local":
		authConf := c.Authentication.Config.(*config.LocalAuthConfig)
		authenticator = auth.NewLocalAuthenticator(authConf, db)
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

	if err := initializeDB(db, c.CreateInitialAdmin); err != nil {
		return err
	}

	resolver := graphqlapi.NewResolver()
	// Since we are using dataloaders to avoid N+1 queries problem we want to
	// parallelize resolvers executions
	s, err := graphql.ParseSchema(graphqlapi.Schema, resolver, graphql.MaxParallelism(1000))
	if err != nil {
		return err
	}

	searchEngine := search.NewSearchEngine(db, c.Index.Path)

	// noop coors handler
	corsHandler := func(h http.Handler) http.Handler {
		return h
	}

	if len(c.Web.AllowedOrigins) > 0 {
		corsAllowedHeadersOptions := ghandlers.AllowedHeaders([]string{"Accept", "Accept-Encoding", "Authorization", "Content-Length", "Content-Type", "X-CSRF-Token", "Authorization"})
		corsAllowedOriginsOptions := ghandlers.AllowedOrigins(c.Web.AllowedOrigins)
		corsHandler = ghandlers.CORS(corsAllowedHeadersOptions, corsAllowedOriginsOptions)
	}

	loginHandler := handlers.NewLoginHandler(c, db, authenticator, memberProvider, tokenSigningData)
	refreshTokenHandler := handlers.NewRefreshTokenHandler(tokenSigningData)
	oidcAuthURLHandler := handlers.NewOIDCAuthURLHandler(authenticator)
	graphqlHandler := handlers.NewGraphQLHandler(c, db, searchEngine, s, memberProvider)
	authHandler := handlers.NewAuthHandler(db, tokenSigningData)

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
	apirouter.Handle("/avatar/{memberuid}", handlers.NewAvatarHandler(db))

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
			listenErrChan <- fmt.Errorf("listening on %s failed: %v", c.Web.HTTP, err)
		}()
	}
	if c.Web.HTTPS != "" {
		log.Infof("https listening on %s", c.Web.HTTPS)
		go func() {
			err := http.ListenAndServeTLS(c.Web.HTTPS, c.Web.TLSCert, c.Web.TLSKey, mainrouter)
			listenErrChan <- fmt.Errorf("listening on %s failed: %v", c.Web.HTTPS, err)
		}()
	}

	return <-listenErrChan
}

func initializeDB(db *db.DB, createInitialAdmin bool) error {
	tx, err := db.NewTx()
	if err != nil {
		return err
	}
	if err := doInit(tx, createInitialAdmin); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func doInit(tx *db.Tx, createInitialAdmin bool) error {
	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		return err
	}
	commandService := command.NewCommandService(tx, readDB, nil, nil, false)

	if !readDB.CurTimeLine().IsZero() {
		return nil
	}

	_, err = commandService.SetupRootRole()
	if err != nil {
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

		ctx := context.Background()
		if _, _, err := commandService.CreateMemberInternal(ctx, c, false, false); err != nil {
			return err
		}
	}

	return nil
}
