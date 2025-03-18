package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

const name = "bsky"

const version = "0.0.49"

var revision = "HEAD"

type config struct {
	Host     string `json:"host"`
	Handle   string `json:"handle"`
	Password string `json:"password"`
	dir      string
	verbose  bool
	prefix   string
}

type Server struct {
	db             *gorm.DB
	discordSession *discordgo.Session
	logger         *zap.SugaredLogger
}

var (
	AppID       string
	GuildID     string
	ShortcutUrl string
	BotName     string
	ServerURL   string
	LocalPort   string
)

var (
	stopBot = make(chan bool)
)

var Profiles []string

type ProfileConfig struct {
	Config *config `json:"config"`
	Path   string  `json:"path"`
}

var profileConfig = make(map[string]*ProfileConfig)

var logger = zap.NewExample().Sugar()
var server *Server

func InitDBConnection(logger *zap.SugaredLogger) (db *gorm.DB, err error) {
	partLogger := logger.Named("InitDBConnection")
	defer func(partLogger *zap.SugaredLogger) {
		err := partLogger.Sync()
		if err != nil {
			logger.Error(err)
		}
	}(partLogger)

	// DBファイルのオープン
	db, err = openDB(partLogger)
	if err != nil {
		partLogger.Error(err)
		return nil, err
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		partLogger.Error(err)
		return nil, err
	}
	return db, nil
}

func openDB(logger *zap.SugaredLogger) (db *gorm.DB, err error) {
	partLogger := logger.Named("openDB")
	defer func(partLogger *zap.SugaredLogger) {
		err := partLogger.Sync()
		if err != nil {
			logger.Error(err)
		}
	}(partLogger)

	// DBファイルのオープン
	db, err = gorm.Open(sqlite.Open("discord-bot.db"), &gorm.Config{})
	if err != nil {
		partLogger.Error(err)
		return nil, err
	}
	return db, nil
}

func main() {
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			fmt.Println(err)
		}
	}(logger)
	mainLogger := logger.Named("main")
	defer func(logger *zap.SugaredLogger) {
		err := logger.Sync()
		if err != nil {
			logger.Error(err)
		}
	}(mainLogger)

	err := readEnvs(mainLogger)
	if err != nil {
		mainLogger.Errorf("failed to read envs: %s", err)
		os.Exit(1)
	}

	db, err := InitDBConnection(mainLogger)
	if err != nil {
		mainLogger.Error(err)
		panic(err)
	}

	//Discordのセッションを作成
	discord, err := discordgo.New(Token)
	if err != nil {
		mainLogger.Error(err)
		panic(err)
	}
	mainLogger.Info("new")
	discord.Token = Token
	if err != nil {
		mainLogger.Error("Error logging in")
		mainLogger.Error(err)
	}

	discord.AddHandler(onMessageCreate) //全てのWSAPIイベントが発生した時のイベントハンドラを追加

	mainLogger.Info("addhandler")
	// websocketを開いてlistening開始
	err = discord.Open()
	mainLogger.Info("open")
	if err != nil {
		mainLogger.Error(err)
	}
	defer func(discord *discordgo.Session) {
		err := discord.Close()
		if err != nil {
			mainLogger.Error(err)
		}
	}(discord)

	server = &Server{
		db:             db,
		discordSession: discord,
		logger:         mainLogger,
	}

	mainLogger.Info("Configuring HTTP server")
	e := echo.New()
	e.Use(middleware.Logger())
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		mainLogger.Error(err)
	}

	e.Use(middleware.CORS())
	e.GET("/api/post_walk", server.postWalk)
	e.GET("/walk-install", server.installWalk)

	//e.GET("/xrpc/app.bsky.feed.describeFeedGenerator", s.handleDescribeFeedGenerator)
	//e.GET("/.well-known/did.json", s.handleServeDidDoc)
	err = e.Start(":" + LocalPort) //ポート番号指定してね
	if err != nil {
		mainLogger.Error(err)
		return
	}

	mainLogger.Info("Listening...")
	<-stopBot //プログラムが終了しないようロック
	return
}

func readEnvs(logger *zap.SugaredLogger) (err error) {
	partLogger := logger.Named("readEnvs")
	//defer func(partLogger *zap.SugaredLogger) {
	//	err := partLogger.Sync()
	//	if err != nil {
	//		logger.Error(err)
	//	}
	//}(partLogger)

	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		partLogger.Error("DISCORD_BOT_TOKEN is not set")
		return fmt.Errorf("DISCORD_BOT_TOKEN is not set")
	}
	Token += botToken

	AppID = os.Getenv("APPLICATION_ID")
	if AppID == "" {
		partLogger.Error("APPLICATION_ID is not set")
		return fmt.Errorf("APPLICATION_ID is not set")
	}
	GuildID = os.Getenv("GUILD_ID")
	if GuildID == "" {
		partLogger.Error("GUILD_ID is not set")
		return fmt.Errorf("GUILD_ID is not set")
	}
	ShortcutUrl = os.Getenv("SHORTCUT_URL")
	if ShortcutUrl == "" {
		partLogger.Error("SHORTCUT_URL is not set")
		return fmt.Errorf("SHORTCUT_URL is not set")
	}
	BotName = os.Getenv("BOT_NAME")
	if BotName == "" {
		partLogger.Error("BOT_NAME is not set")
		return fmt.Errorf("BOT_NAME is not set")
	}
	ServerURL = os.Getenv("SERVER_URL")
	if ServerURL == "" {
		partLogger.Error("SERVER_URL is not set")
		return fmt.Errorf("SERVER_URL is not set")
	}
	LocalPort = os.Getenv("LOCAL_PORT")
	if LocalPort == "" {
		partLogger.Error("LOCAL_PORT is not set")
		return fmt.Errorf("LOCAL_PORT is not set")
	}
	profiles := os.Getenv("PROFILES")
	if profiles == "" {
		partLogger.Error("PROFILES is not set")
		return fmt.Errorf("PROFILES is not set")
	}
	Profiles = strings.Split(profiles, ",")
	for _, p := range Profiles {
		cfg, fp, err := loadConfig(p)
		if err != nil {
			partLogger.Error(err)
			return err
		}
		cfg.prefix = p + "-"
		profileConfig[p] = &ProfileConfig{
			cfg,
			fp,
		}
	}
	for _, p := range Profiles {
		partLogger.Infof("profile: %s", p)
	}
	return nil
}
