package main

import (
	"fmt"
	v1 "mmchat/internal/api/v1"
	"mmchat/internal/component"
	"mmchat/internal/config"
	"mmchat/internal/middleware"
	"mmchat/internal/repository"
	"mmchat/internal/service"
	"mmchat/internal/zlog"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func newEngine() *gin.Engine {
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		panic(err)
	}
	engine.Use(middleware.RequestID())
	engine.Use(middleware.Logger())
	engine.Use(middleware.Recovery())

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://127.0.0.1:5173"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	engine.Use(cors.New(corsConfig))

	engine.Use(middleware.ErrorHandler())
	engine.Static("/static/avatars", config.GetConfig().StaticAvatarPath)
	engine.Static("/static/files", config.GetConfig().StaticFilePath)
	return engine
}

func bootstrap(cfg *config.Config, db *gorm.DB, rdb *redis.Client) *gin.Engine {
	userRepo := repository.NewUserRepository(db)
	userProfileRepo := repository.NewUserProfileRepository(db)
	mediaRepo := repository.NewMediaRepository(db)

	smsService := service.NewSmsService(
		rdb,
		service.NewSender(),
		cfg.AuthConfig.RegisterCodeTTL.Duration,
		cfg.AuthConfig.RegisterCodeResendInterval.Duration,
		cfg.AuthConfig.RegisterCodePhoneHourlyLimit)
	refreshTokenProvider := service.NewRefreshTokenProvider()
	accessTokenProvider, err := service.NewAccessTokenProvider(cfg.AuthConfig.Issuer, os.Getenv("MOCHA_JWT_SECRET"), cfg.AuthConfig.AccessTokenTTL.Duration)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init access token provider: %s", err))
	}
	sessionService := service.NewSessionService(rdb, cfg.AuthConfig.RefreshTokenTTL.Duration)
	authService := service.NewAuthService(
		smsService, userRepo, userProfileRepo, refreshTokenProvider, accessTokenProvider, sessionService)

	objs, err := service.NewObjectStorageService(&cfg.MinIOConfig)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init object storage service: %s", err))
	}
	mediaService := service.NewMediaService(objs, userRepo, mediaRepo)
	userService := service.NewUserService(userRepo, userProfileRepo, mediaRepo, objs)

	engine := newEngine()
	authentication := middleware.Authentication(accessTokenProvider, sessionService)
	{
		apiv1 := engine.Group("/api/v1")
		v1.NewAuthHandler(authService, authentication).RegisterRoutes(apiv1.Group("/auth"))
		v1.NewUserHandler(userService, authentication).RegisterRoutes(apiv1.Group("/users"))
		v1.NewMediaHandler(mediaService, authentication).RegisterRoutes(apiv1.Group("/media"))
	}

	return engine
}

func main() {
	cfg := config.GetConfig()
	db, err := component.InitMysql(&cfg.MysqlConfig)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init Mysql error: %s", err))
	}
	defer func() {
		cn, _ := db.DB()
		cn.Close()
	}()

	rdb, err := component.InitRedis(&cfg.RedisConfig)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init Redis error: %s", err))
	}
	defer rdb.Close()

	engine := bootstrap(cfg, db, rdb)

	addr := fmt.Sprintf("%s:%d", cfg.MainConfig.Host, cfg.MainConfig.Port)
	zlog.Info("server listening at: " + addr)
	if err := engine.Run(addr); err != nil {
		zlog.Fatal(fmt.Sprintf("run engine error: %s", err))
	}

}
