package server

import (
	"github.com/BurntSushi/toml"
	"github.com/devhoodit/sse-chat/auth"
	githubAuth "github.com/devhoodit/sse-chat/auth/github"
	"github.com/devhoodit/sse-chat/database"
	"github.com/devhoodit/sse-chat/store"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

var SecretKey *secret

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// load config form conf.toml
	var conf conf
	_, err := toml.DecodeFile("conf.toml", &conf)
	if err != nil {
		panic(err)
	}

	// database setting
	d, err := database.New(conf.Database.Username, conf.Database.Password, conf.Database.Address, conf.Database.Port)
	if err != nil {
		panic(err)
	}
	err = d.AutoMigration() // auto migration, check table is Exist, if not create
	if err != nil {
		panic(err)
	}

	//redis setting
	store, err := store.New(&redis.Options{
		Addr:     conf.Store.Address,
		Password: conf.Store.Password, // no password set
		DB:       conf.Store.Db,       // use default DB
	})
	if err != nil {
		panic(err)
	}

	// MiddleWare setting, server/middleware.go
	setMiddleWare(r, &conf)

	jwtAuth := &auth.JwtAuth{
		Secret: conf.Secret.Jwtkey,
		DB:     d,
		Store:  store,
	}

	// github Oauth router
	githubRouter := githubAuth.Github{
		DB:      d,
		JwtAuth: jwtAuth,
		OAuthConfig: &oauth2.Config{
			ClientID:     conf.Oauth["github"].Id,
			ClientSecret: conf.Oauth["github"].Secret,
			RedirectURL:  conf.Oauth["github"].Redirect,
			Scopes:       []string{"user:email"},
			Endpoint:     github.Endpoint,
		}}

	r.GET("/", func(c *gin.Context) {
	})

	authRouter := r.Group("/auth")
	{
		sse := authRouter.Group("/sse")
		{
			sse.GET("/login")
			sse.POST("/create")
		}
		github := authRouter.Group("/github")
		{
			github.GET("/login", githubRouter.Login)
			github.GET("/callback", githubRouter.Callback)
		}
		r.GET("/token").Use().Use()
	}

	return r
}
