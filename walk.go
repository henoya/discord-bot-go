package main

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"

	"go.uber.org/zap"
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

func (s *Server) postWalk(e echo.Context) error {
	partLogger := s.logger.Named("postWalk")
	defer func(partLogger *zap.SugaredLogger) {
		err := partLogger.Sync()
		if err != nil {
			s.logger.Error(err)
		}
	}(partLogger)

	_ = e.Request().Context()
	hash := e.QueryParam("id")
	walk := e.QueryParam("walk")
	today := e.QueryParam("today")
	yesterday := e.QueryParam("yesterday")
	yesterdayWalk := e.QueryParam("yesterday_walk")
	partLogger.Infof("hash: %s  today: %s walk: %s, yesterday: %s yesterday_walk: %s", hash, today, walk, yesterday, yesterdayWalk)

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
			partLogger.Errorf("failed to found user: %w", err)
			return e.JSON(http.StatusUnauthorized, fmt.Errorf("failed to found user: %w", err))
		}

		authedUser = &u
	}
	if authedUser == nil {
		err := echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
		partLogger.Errorf("failed to found user: %w", err)
		return err
	}
	//if m.Member != nil {
	//	fmt.Printf("Member: %s\n", m.Member.Nick)
	//}

	message := ""
	if yesterday != "" && yesterdayWalk != "" {
		message = fmt.Sprintf("%s さんは、\n昨日 %s 日 %s 歩\n今日 %s 日はこれまで %s 歩\n歩きました！",
			authedUser.Name, yesterday, yesterdayWalk, today, walk)
	} else {
		message = fmt.Sprintf("%s さんは、今日 %s 歩歩きました！", authedUser.Name, walk)
	}
	partLogger.Infof("%s  %s", authedUser.ChannelId, message)
	err := sendMessage(partLogger, s.discordSession, authedUser.ChannelId, message)
	if err != nil {
		partLogger.Error(err)
		return e.JSON(http.StatusInternalServerError, fmt.Errorf("failed to send message: %w", err))
	}
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
