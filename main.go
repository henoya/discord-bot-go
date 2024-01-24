package main

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/bwmarrin/discordgo"
	logging "github.com/ipfs/go-log"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	gorm "gorm.io/gorm"
)

type Server struct {
	db             *gorm.DB
	discordSession *discordgo.Session
}

// import (
//
//	"time"
//
//	"github.com/henoya/sorascope/enum"
//	"github.com/henoya/sorascope/typedef"
//
//	_ "github.com/mattn/go-sqlite3"
//
// )
var (
	AppID       string
	GuildID     string
	ShortcutUrl string
	BotName     string
	ServerURL   string
	LocalPort   string
)

type User struct {
	Id          string `json:"id" gorm:"type:text;primary_key"`
	Name        string `json:"name" gorm:"type:text"`
	GuildId     string `json:"guild_id" gorm:"type:text"`
	ChannelId   string `json:"channel_id" gorm:"type:text"`
	HashCode    string `json:"hash_code" gorm:"type:text"`
	InstallHash string `json:"install_hash" gorm:"type:text"`
	Enable      bool   `json:"enable" gorm:"type:boolean"`
}

var (
	Token             = "Bot " //"Bot"という接頭辞がないと401 unauthorizedエラーが起きます
	stopBot           = make(chan bool)
	vcsession         *discordgo.VoiceConnection
	HelpCommand       = "!help"
	InstallCmd        = "!インストール"
	PostHear          = "!ここにポスト"
	ChannelVoiceLeave = "!vcleave"
)

var log = logging.Logger("discord-bot")

var server *Server

func InitDBConnection() (db *gorm.DB, err error) {
	// DBファイルのオープン
	db, err = openDB()
	if err != nil {
		return nil, fmt.Errorf("failed to connect database")
	}

	if err := db.AutoMigrate(&User{}); err != nil {
		panic(err)
	}
	return db, nil
}

func openDB() (db *gorm.DB, err error) {
	// DBファイルのオープン
	db, err = gorm.Open(sqlite.Open("discord-bot.db"), &gorm.Config{})
	return db, err
}

func main() {
	botToken := os.Getenv("DISCORD_BOT_TOKEN")
	if botToken == "" {
		log.Error("DISCORD_BOT_TOKEN is not set")
		panic("DISCORD_BOT_TOKEN is not set")
	}
	Token += botToken

	AppID = os.Getenv("APPLICATION_ID")
	if AppID == "" {
		log.Error("APPLICATION_ID is not set")
		panic("APPLICATION_ID is not set")
	}
	GuildID = os.Getenv("GUILD_ID")
	if GuildID == "" {
		log.Error("GUILD_ID is not set")
		panic("GUILD_ID is not set")
	}
	ShortcutUrl = os.Getenv("SHORTCUT_URL")
	if ShortcutUrl == "" {
		log.Error("SHORTCUT_URL is not set")
		panic("SHORTCUT_URL is not set")
	}
	BotName = os.Getenv("BOT_NAME")
	if BotName == "" {
		log.Error("BOT_NAME is not set")
		panic("BOT_NAME is not set")
	}
	ServerURL = os.Getenv("SERVER_URL")
	if ServerURL == "" {
		log.Error("SERVER_URL is not set")
		panic("SERVER_URL is not set")
	}
	LocalPort = os.Getenv("LOCAL_PORT")
	if LocalPort == "" {
		log.Error("LOCAL_PORT is not set")
		panic("LOCAL_PORT is not set")
	}

	//Discordのセッションを作成
	discord, err := discordgo.New(Token)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	fmt.Println("new")
	discord.Token = Token
	if err != nil {
		log.Error("Error logging in")
		log.Error(err)
	}

	discord.AddHandler(onMessageCreate) //全てのWSAPIイベントが発生した時のイベントハンドラを追加

	log.Info("addhandler")
	// websocketを開いてlistening開始
	err = discord.Open()
	log.Info("open")
	if err != nil {
		log.Error(err)
	}
	defer discord.Close()

	db, err := InitDBConnection()
	if err != nil {
		log.Error(err)
		panic(err)
	}

	log.Infof("Configuring HTTP server")
	e := echo.New()
	e.Use(middleware.Logger())
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		log.Error(err)
	}

	server = &Server{
		db:             db,
		discordSession: discord,
	}

	e.Use(middleware.CORS())
	e.GET("/api/post_walk", server.postWalk)
	e.GET("/walk-install", server.installWalk)

	//e.GET("/xrpc/app.bsky.feed.describeFeedGenerator", s.handleDescribeFeedGenerator)
	//e.GET("/.well-known/did.json", s.handleServeDidDoc)
	err = e.Start(":" + LocalPort) //ポート番号指定してね
	if err != nil {
		log.Error(err)
		return
	}
	log.Info("Listening...")
	<-stopBot //プログラムが終了しないようロック
	return
}

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	//if err != nil {
	//    log.Println("Error getting channel: ", err)
	//    return
	//}
	log.Infof("ChannelID: %20s time: %20s Username: %20s > content: %s\n", m.ChannelID, time.Now().Format(time.Stamp), m.Author.Username, m.Content)
	log.Infof("GuildID: %20s Author.ID: %20s Author.Token: %20s\n", m.GuildID, m.Author.ID, m.Author.Token)
	ch, err := s.Channel(m.ChannelID)
	if err != nil {
		log.Error(err)
		return
	}
	channelName := ch.Name
	if channelName == "" {
		channelName = ch.ID
	}
	switch {
	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, HelpCommand)):
		sendMessage(s, m.ChannelID, fmt.Sprint("コマンド一覧です\n"+
			"- "+InstallCmd+": インストール\n"+
			"- "+PostHear+": 歩数のポストを現在のチャンネルに指定する\n"+
			"- "+HelpCommand+": このメッセージ"))

	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, InstallCmd)):
		seed := m.Author.ID + (strconv.Itoa(rand.Int()) + m.Author.ID + m.Content)
		sum := sha512.Sum512([]byte(seed))
		sumByte := sum[:]
		hashCode := hex.EncodeToString(sumByte)
		user, _ := server.getOrCreateUser(nil, m.Author.ID)
		if user == nil {
			user = &User{
				Id:          m.Author.ID,
				Name:        m.Author.Username,
				GuildId:     m.GuildID,
				ChannelId:   "",
				HashCode:    hashCode,
				InstallHash: "",
				Enable:      true,
			}
			if err := server.db.Create(&user).Error; err != nil {
				log.Error(err)
				return
			}
		} else {
			user.HashCode = hashCode
			if err := server.db.Save(&user).Error; err != nil {
				log.Error(err)
				return
			}
		}
		sendMessage(s, m.ChannelID, fmt.Sprintf("はい、では次のURLにアクセスして、iPhone 用のショートカットをダウンロードして、IDを設定してください。\n"+
			"%s/walk-install?code=%s",
			ServerURL, hashCode))

	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, PostHear)):
		user, _ := server.getOrCreateUser(nil, m.Author.ID)
		if user == nil || user.Id == "" || user.HashCode != "" {
			sendMessage(s, m.ChannelID, fmt.Sprintf("まだ、%s さんは、初期設定を完了していないようです。\n"+
				"まずは !インストール コマンドを実行してください。", m.Author.Username))
			return
		}
		user.ChannelId = m.ChannelID
		user.GuildId = m.GuildID
		server.db.Save(&user)
		sendMessage(s, m.ChannelID, fmt.Sprintf("はい、%s さんの一日の歩数報告を、ここ %s チャンネルにポストします。",
			m.Author.Username, channelName))

	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, ChannelVoiceLeave)):
		vcsession.Disconnect() //今いる通話チャンネルから抜ける
	}
}

// メッセージを送信する関数
func sendMessage(s *discordgo.Session, channelID string, msg string) {
	fmt.Printf("%s %s\n", channelID, msg)
	_, err := s.ChannelMessageSend(channelID, msg)

	log.Infof(">>> %s", msg)
	if err != nil {
		log.Errorf("Error sending message: %s", err)
	}
}

func (s *Server) postWalk(e echo.Context) error {
	_ = e.Request().Context()
	hash := e.QueryParam("id")
	walk := e.QueryParam("walk")
	log.Infof("hash: %s   walk: %s", hash, walk)

	var authedUser *User
	//if auth := e.Request().Header.Get("Authorization"); auth != "" {
	//	parts := strings.Split(auth, " ")
	//	if parts[0] != "Bearer" || len(parts) != 2 {
	//		return fmt.Errorf("invalid auth header")
	//	}
	if hash != "" && walk != "" {
		var u User
		err := s.db.Find(&u, "install_hash = ?", hash).Error
		if err != nil {
			return e.JSON(http.StatusUnauthorized, fmt.Errorf("failed to found user: %w", err))
		}

		authedUser = &u
	}
	if authedUser == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}
	fmt.Printf("%s  %s", authedUser.ChannelId, fmt.Sprintf("%s さんは、今日 %s 歩歩きました！", authedUser.Name, walk))
	sendMessage(s.discordSession, authedUser.ChannelId, fmt.Sprintf("%s さんは、今日 %s 歩歩きました！", authedUser.Name, walk))
	out := "ok"
	return e.JSON(200, out)
}

func (s *Server) installWalk(e echo.Context) error {
	_ = e.Request().Context()
	h := e.Request().Header.Get("User-Agent")
	if strings.Contains(h, "https://discordapp.com") {
		return echo.NewHTTPError(http.StatusForbidden, "Forbidden")
	}
	code := e.QueryParam("code")
	fmt.Printf("code: %s\n", code)

	var u User

	if err := s.db.Find(&u, "hash_code = ?", code).Error; err != nil {
		html := fmt.Sprintf("<html><body><h1>%s</h1></body></html>", "DBエラーです")
		return e.HTML(500, html)
	}
	if u.Id == "" {
		html := fmt.Sprintf("<html><body><h1>%s</h1></body></html>", "コードが見つかりません")
		return e.HTML(403, html)
	}

	seed := u.Id + (strconv.Itoa(rand.Int()) + u.Id)
	sum := sha512.Sum512([]byte(seed))
	sumByte := sum[:]
	hashCode := hex.EncodeToString(sumByte)

	u.InstallHash = hashCode
	u.HashCode = ""
	s.db.Save(&u)
	html := fmt.Sprintf("<!DOCTYPE html>"+
		"<html>"+
		"<head>"+
		"<meta charSet=\"utf-8\"/>"+
		"<meta name=\"viewport\" content=\"width=device-width\"/>"+
		"<title>ショートカットインストール</title>"+
		"</head>"+
		"<body>"+
		"<h1>%s</h1>"+
		"<p>%s</p>"+
		"<h3><a href=\"%s\">歩数報告ショートカットをインストール</a></h3>"+
		"    <!-- コピー対象要素とコピーボタン -->"+
		"    <input id=\"copyTarget\" type=\"text\" style=\"\" value=\"%s\" readonly>"+
		"</body></html>",
		"ショートカットインストール",
		"以下のURLから、ショートカットをインストールして、<br/>下のコードをコピーして設定してください。",
		ShortcutUrl,
		hashCode)
	return e.HTML(200, html)
}

func (s *Server) getOrCreateUser(ctx context.Context, uid string) (*User, error) {
	var u User

	fmt.Printf("s: %v\n", s)
	fmt.Printf("db: %v\n", s.db)
	if err := s.db.Find(&u, "id = ?", uid).Error; err != nil {
		return nil, err
	}
	if u.Id == "" {
		return nil, fmt.Errorf("user not found")
	}

	return &u, nil
}
