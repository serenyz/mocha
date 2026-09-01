package main

import (
	"context"
	"fmt"
	"os"

	v1 "mmchat/internal/api/v1"
	"mmchat/internal/component"
	"mmchat/internal/config"
	"mmchat/internal/messaging"
	"mmchat/internal/middleware"
	"mmchat/internal/repository"
	"mmchat/internal/service"
	"mmchat/internal/websocket"
	"mmchat/internal/worker"
	"mmchat/internal/zlog"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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

func bootstrap(
	ctx context.Context,
	cfg *config.Config,
	db *gorm.DB,
	rdb *redis.Client,
	kafkaClients *component.KafkaClients,
) *gin.Engine {
	userRepo := repository.NewUserRepository(db)
	userProfileRepo := repository.NewUserProfileRepository(db)
	mediaRepo := repository.NewMediaRepository(db)
	conversationRepo := repository.NewConversationRepository(db)
	messageRepo := repository.NewMessageRepository(db)

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
	webSocketTicketService, err := service.NewWebSocketTicketService(
		rdb,
		sessionService,
		cfg.WebSocketConfig.TicketTTL.Duration,
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init WebSocket ticket service: %s", err))
	}

	objs, err := service.NewObjectStorageService(&cfg.MinIOConfig, rdb)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init object storage service: %s", err))
	}
	mediaService := service.NewMediaService(objs, mediaRepo)
	userService := service.NewUserService(userRepo, userProfileRepo, mediaRepo, objs)

	conversationService := service.NewConversationService(conversationRepo, messageRepo, userRepo, mediaRepo, objs)
	messagePublisher, err := messaging.NewMessageCommandPublisher(
		kafkaClients.CommandProducer,
		cfg.KafkaConfig.CommandTopic,
		cfg.KafkaConfig.ProduceTimeout.Duration,
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init message publisher: %s", err))
	}
	messageOutcomePublisher, err := messaging.NewMessageOutcomePublisher(
		kafkaClients.OutcomeProducer,
		cfg.KafkaConfig.CommittedTopic,
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init message outcome publisher: %s", err))
	}
	messagePushPublisher, err := messaging.NewMessagePushPublisher(rdb, cfg.RedisConfig.MessageChannel)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init message push publisher: %s", err))
	}
	messageService := service.NewMessageService(
		messagePublisher,
		messagePushPublisher,
		messageRepo,
		conversationRepo,
		mediaRepo,
		objs,
	)
	messageWriter, err := worker.NewMessageWriter(
		kafkaClients.MessageWriter,
		messageOutcomePublisher,
		messageService,
		cfg.KafkaConfig.BatchSize,
		cfg.KafkaConfig.WriterTimeout.Duration,
		cfg.KafkaConfig.WriterRetry.Duration,
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init message writer: %s", err))
	}
	go func() {
		if err := messageWriter.Run(ctx); err != nil {
			zlog.Error("message writer stopped", zap.Error(err))
		}
	}()

	hub := websocket.NewHub()
	messagePush, err := worker.NewMessagePush(
		kafkaClients.MessagePush,
		messagePushPublisher,
		messageService,
		cfg.KafkaConfig.BatchSize,
		cfg.KafkaConfig.PushTimeout.Duration,
		cfg.KafkaConfig.PushRetry.Duration,
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init message push: %s", err))
	}
	messageSubscriber, err := websocket.NewMessageSubscriber(
		rdb,
		cfg.RedisConfig.MessageChannel,
		hub,
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init message subscriber: %s", err))
	}
	go func() {
		if err := messageSubscriber.Run(ctx); err != nil {
			zlog.Error("message subscriber stopped", zap.Error(err))
		}
	}()
	go func() {
		if err := messagePush.Run(ctx); err != nil {
			zlog.Error("message push stopped", zap.Error(err))
		}
	}()

	webSocketServer, err := websocket.NewServer(
		&cfg.WebSocketConfig,
		webSocketTicketService,
		hub,
		websocket.NewMessageEventHandler(messageService),
	)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init WebSocket server: %s", err))
	}

	engine := newEngine()
	authentication := middleware.Authentication(accessTokenProvider, sessionService)
	{
		apiv1 := engine.Group("/api/v1")
		apiv1.GET("/ws", gin.WrapH(webSocketServer))
		v1.NewWebSocketTicketHandler(webSocketTicketService, authentication).RegisterRoutes(apiv1.Group("/ws"))
		v1.NewAuthHandler(authService, authentication).RegisterRoutes(apiv1.Group("/auth"))
		v1.NewUserHandler(userService, authentication).RegisterRoutes(apiv1.Group("/users"))
		v1.NewMediaHandler(mediaService, authentication).RegisterRoutes(apiv1.Group("/media"))
		v1.NewConversationHandler(conversationService, messageService, authentication).
			RegisterRoutes(apiv1.Group("/conversations"))
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

	kafkaClients, err := component.InitKafka(&cfg.KafkaConfig)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("Init Kafka error: %s", err))
	}
	defer kafkaClients.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := bootstrap(ctx, cfg, db, rdb, kafkaClients)

	addr := fmt.Sprintf("%s:%d", cfg.MainConfig.Host, cfg.MainConfig.Port)
	zlog.Info("server listening at: " + addr)
	if err := engine.Run(addr); err != nil {
		zlog.Fatal(fmt.Sprintf("run engine error: %s", err))
	}

}
