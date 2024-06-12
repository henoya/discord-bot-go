package main

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	_ "github.com/mattn/go-sqlite3"

	"go.uber.org/zap"
)

var (
	Token             = "Bot " //"Bot"という接頭辞がないと401 unauthorizedエラーが起きます
	vcsession         *discordgo.VoiceConnection
	HelpCommand       = "!help"
	InstallCmd        = "!インストール"
	PostHear          = "!ここにポスト"
	UrlPost           = "!URLポスト"
	ChannelVoiceLeave = "!vcleave"
)

func onMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	partLogger := logger.Named("onMessageCreate")
	handle := m.Author.ID
	if m.Author != nil {
		if m.Author.Username != "" {
			handle = m.Author.Username
		} else {
			partLogger.Infof("AuthoID: %s Username: is Empty use Author.ID %s\n", m.Author.ID, m.Author.ID)
			handle = m.Author.ID
		}
	}
	partLogger.Infof("Try get UserID %s Handle. get %s", m.Author.ID, handle)
	nickName := handle
	gm, err := s.GuildMember(m.GuildID, m.Author.ID)
	if err != nil {
		nickName = handle
		partLogger.Warnf("GuildMember error: use nickName is handle %s\n", handle)
		partLogger.Error(err)
	} else if gm != nil {
		if gm.Nick != "" {
			nickName = gm.Nick
		} else {
			us, err := s.User(m.Author.ID)
			_ = us
			if err != nil {
				nickName = handle
				partLogger.Warnf("User error: use nickName is handle %s\n", handle)
				partLogger.Error(err)
			}
			nickName = handle
			partLogger.Infof("AuthoID: %s NickName: is Empty use handle %s\n", m.Author.ID, handle)
		}
	}
	partLogger.Infof("Try get UserID %s NickName. get %s", m.Author.ID, nickName)
	//defer func(partLogger *zap.SugaredLogger) {
	//	err := partLogger.Sync()
	//	if err != nil {
	//		logger.Error(err)
	//	}
	//}(partLogger)

	//if err != nil {
	//    log.Println("Error getting channel: ", err)
	//    return
	//}
	partLogger.Infof("ChannelID: %20s time: %20s Username: %20s > content: %s\n", m.ChannelID, time.Now().Format(time.Stamp), m.Author.Username, m.Content)
	partLogger.Infof("GuildID: %20s Author.ID: %20s Author.Token: %20s\n", m.GuildID, m.Author.ID, m.Author.Token)
	ch, err := s.Channel(m.ChannelID)
	if err != nil {
		partLogger.Error(err)
		return
	}
	channelName := ch.Name
	if channelName == "" {
		channelName = ch.ID
	}
	switch {
	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, HelpCommand)):
		err = sendMessage(partLogger, s, m.ChannelID, fmt.Sprint("コマンド一覧です\n"+
			"- "+InstallCmd+": インストール\n"+
			"- "+PostHear+": 歩数のポストを現在のチャンネルに指定する\n"+
			"- "+HelpCommand+": このメッセージ"))
		if err != nil {
			partLogger.Error(err)
			return
		}
		return

	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, InstallCmd)):
		seed := m.Author.ID + (strconv.Itoa(rand.Int()) + m.Author.ID + m.Content)
		sum := sha512.Sum512([]byte(seed))
		sumByte := sum[:]
		hashCode := hex.EncodeToString(sumByte)
		user, _ := server.getOrCreateUser(nil, m.Author.ID)
		if user == nil {
			user = &User{
				Id:          m.Author.ID,
				Name:        nickName,
				GuildId:     m.GuildID,
				ChannelId:   "",
				HashCode:    hashCode,
				InstallHash: "",
				Enable:      true,
			}
			if err := server.db.Create(&user).Error; err != nil {
				partLogger.Error(err)
				return
			}
		} else {
			user.HashCode = hashCode
			if err := server.db.Save(&user).Error; err != nil {
				partLogger.Error(err)
				return
			}
		}
		err = sendMessage(partLogger, s, m.ChannelID, fmt.Sprintf("はい、では次のURLにアクセスして、iPhone 用のショートカットをダウンロードして、IDを設定してください。\n"+
			"%s/walk-install?code=%s",
			ServerURL, hashCode))
		if err != nil {
			partLogger.Error(err)
			return
		}
		return

	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, PostHear)):
		user, _ := server.getOrCreateUser(nil, m.Author.ID)
		if user == nil || user.Id == "" || user.HashCode != "" {
			err = sendMessage(partLogger, s, m.ChannelID, fmt.Sprintf("まだ、%s さんは、初期設定を完了していないようです。\n"+
				"まずは !インストール コマンドを実行してください。", nickName))
			if err != nil {
				partLogger.Error(err)
				return
			}
			return
		}
		user.ChannelId = m.ChannelID
		user.GuildId = m.GuildID
		user.Name = nickName
		server.db.Save(&user)
		err = sendMessage(partLogger, s, m.ChannelID, fmt.Sprintf("はい、%s さんの一日の歩数報告を、ここ %s チャンネルにポストします。",
			nickName, channelName))
		if err != nil {
			partLogger.Error(err)
			return
		}
		return

	case strings.HasPrefix(m.Content, fmt.Sprintf("%s %s", BotName, UrlPost)):
		PostUrl := m.Content[len(fmt.Sprintf("%s %s", BotName, UrlPost)):]
		PostUrl = strings.TrimSpace(PostUrl)
		otherChannel := ""
		if strings.Contains(PostUrl, " channel ") {
			otherChannel = strings.Split(PostUrl, " channel ")[1]
			PostUrl = strings.Split(PostUrl, " channel ")[0]
		}
		if PostUrl == "" {
			partLogger.Info("URLを指定してください。")
			err = sendMessage(partLogger, s, m.ChannelID, fmt.Sprintf("URLを指定してください。"))
			if err != nil {
				partLogger.Error(err)
				return
			}
			return
		}
		_, err := url.Parse(PostUrl)
		if err != nil {
			partLogger.Error(err)
			_ = sendMessage(partLogger, s, m.ChannelID, fmt.Sprintf("URLが不正です。"))
			return
		}
		doc, err := fetchHTML(PostUrl)
		if err != nil {
			partLogger.Errorf("Error fetching URL:", err)
			return
		}
		ogpData := extractOGP(doc)
		if ogpData == nil {
			partLogger.Error("Error extracting OGP data")
			return
		}
		partLogger.Infof("Title: %s\n", ogpData.Title)
		partLogger.Infof("Type: %s\n", ogpData.Type)
		partLogger.Infof("Image: %s\n", ogpData.Image)
		partLogger.Infof("URL: %s\n", ogpData.URL)
		partLogger.Infof("Description: %s\n", ogpData.Description)
		//Title Henoya🍄Shiitake (@henoya.com)
		//Type article
		//Image https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:trw6iydbhpncolfzwrrh5juw/bafkreigbxhvh4tgylwmocwpwfvrfbbdcu5dcw43nkave67rjat6yzhseue@jpeg
		//URL https://bsky.app/profile/henoya.com/post/3kkcu4mpudm2n
		//Description おはようございます。 2024/01/31 23:54 から 2024/02/01 07:40 まで 7時間 46分の睡眠でした 睡眠の評価は 73% 睡眠: 6:37 深い睡眠: 1:25 良質な睡眠: 3:37
		if ogpData.Title == "" && ogpData.URL == "" {
			partLogger.Warn("タイトルが取得できませんでした。")
			err = sendMessage(partLogger, s, m.ChannelID, fmt.Sprintf("タイトルが取得できませんでした。"))
			if err != nil {
				partLogger.Error(err)
				return
			}
			return
		}
		embed := discordgo.MessageEmbed{
			Type:        "rich",
			Title:       PostUrl,
			URL:         PostUrl,
			Description: ogpData.Description,
			Author: &discordgo.MessageEmbedAuthor{
				Name: ogpData.Title,
			},
		}
		if ogpData.Image != "" {
			embedImage := &discordgo.MessageEmbedImage{
				URL: ogpData.Image,
			}
			embed.Image = embedImage
		}
		msgData := &discordgo.MessageSend{
			Components: nil,
			Embeds:     []*discordgo.MessageEmbed{&embed},
			TTS:        false,
		}
		msg, err := json.Marshal(msgData)
		if err != nil {
			partLogger.Error(err)
			return
		}
		//msg := "{\n" +
		//	//"    \"username\": \"Henoya🍄Shiitake @henoya.com\",\n" +
		//	//"    \"avatar_url\": \"https://cdn.bsky.app/img/avatar/plain/did:plc:trw6iydbhpncolfzwrrh5juw/bafkreicxy3lslhwua4pvghkfkjrs77daoypkzkqwpuae7ozh6qgegtv4ty@jpeg\",\n" +
		//	//"    \"content\":\"はい、henoya チャンネルにポストします。\"," +
		//	"    \"embeds\": [\n" +
		//	"        {\n" +
		//	"            \"type\": \"rich\",\n" +
		//	"            \"title\": \"" + PostUrl + "\",\n" +
		//	"            \"image\": {\n" +
		//	"                \"url\": \"https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:trw6iydbhpncolfzwrrh5juw/bafkreigbxhvh4tgylwmocwpwfvrfbbdcu5dcw43nkave67rjat6yzhseue@jpeg\",\n" +
		//	"                \"height\": 0,\n" +
		//	"                \"width\": 0\n" +
		//	"            },\n" +
		//	"            \"author\": {\n" +
		//	"                \"name\": \"Henoya🍄Shiitake @henoya.com\",\n" +
		//	"                \"url\": \"https://bsky.app/profile/henoya.com\",\n" +
		//	"                \"icon_url\": \"https://cdn.bsky.app/img/avatar/plain/did:plc:trw6iydbhpncolfzwrrh5juw/bafkreicxy3lslhwua4pvghkfkjrs77daoypkzkqwpuae7ozh6qgegtv4ty@jpeg\"\n" +
		//	"            },\n" +
		//	"            \"footer\": {\n" +
		//	"                \"text\": \"おはようございます。\\n2024/01/31 23:54 から 2024/02/01 07:40 まで 7時間 46分の睡眠でした\\n睡眠の評価は 73%\\n睡眠: 6:37\\n深い睡眠: 1:25\\n良質な睡眠: 3:37\"\n" +
		//	"            },\n" +
		//	"            \"url\": \"https://bsky.app/profile/henoya.com/post/3kkcu4mpudm2n\"\n" +
		//	"        }\n" +
		//	"    ],\n" +
		//	//"    \"embeds\":null,\n" +
		//	"    \"tts\":false,\n" +
		//	"    \"components\":null" +
		//	"}"
		//msg = "{\"content\":\"はい、henoya チャンネルにポストします。\",\"embeds\":null,\"tts\":false,\"components\":null}"
		//msg = "{\"content\":\"はい、henoya チャンネルにポストします。\",\"embeds\":null,\"tts\":false,\"components\":null}"
		//msg = `{"content":"はい、henoya チャンネルにポストします。","embeds":null,"tts":false,"components":null}`
		//sendMessage(s, m.ChannelID, fmt.Sprintf("はい、%s チャンネルにポストします。", channelName))
		endpoint := discordgo.EndpointChannelMessages(m.ChannelID)
		b := []byte(msg)
		response, err := s.RequestWithLockedBucket("POST", endpoint, "application/json", b, s.Ratelimiter.LockBucket(endpoint), 0, []discordgo.RequestOption{}...)
		if err != nil {
			partLogger.Error(err)
			return
		}
		partLogger.Infof("response: %s\n", string(response))

		if otherChannel != "" {
			otherEndpoint := discordgo.EndpointChannelMessages(otherChannel)
			otherb := []byte(msg)
			otherResponse, err := s.RequestWithLockedBucket("POST", otherEndpoint, "application/json", otherb, s.Ratelimiter.LockBucket(otherEndpoint), 0, []discordgo.RequestOption{}...)
			if err != nil {
				partLogger.Error(err)
				return
			}
			partLogger.Infof("otherResponse: %s\n", string(otherResponse))
		}
		//response, err := s.request("POST", endpoint, "application/json", msg, endpoint, 0, []discordgo.RequestOption{}...)
		//{
		//  "embeds":[
		//   {
		//      "url":"https://bsky.app/profile/henoya.com/post/3kkcu4mpudm2n",
		//      "type":"rich",
		//      "title":"Henoya🍄Shiitake (@henoya.com)",
		//      "description":"おはようございます。\n2024/01/31 23:54 から 2024/02/01 07:40 まで 7時間 46分の睡眠でした\n睡眠の評価は 73%\n睡眠: 6:37\n深い睡眠: 1:25\n良質な睡眠: 3:37 おはようございます。",
		//      "timestamp":"Feb  4 01:57:07",
		//      "footer":{
		//        "text":"おはようございます。\n2024/01/31 23:54 から 2024/02/01 07:40 まで 7時間 46分の睡眠でした\n睡眠の評価は 73%\n睡眠: 6:37\n深い睡眠: 1:25\n良質な睡眠: 3:37 おはようございます。"
		//      },
		//      "image":{
		//        "url":"https://cdn.bsky.app/img/feed_thumbnail/plain/did:plc:trw6iydbhpncolfzwrrh5juw/bafkreigbxhvh4tgylwmocwpwfvrfbbdcu5dcw43nkave67rjat6yzhseue@jpeg"
		//      },
		//      "author":{
		//        "name":"Henoya🍄Shiitake (@henoya.com)"
		//      }
		//    }
		//  ],
		//  "tts":false,
		//  "components":null
		// }

		//response, err := s.RequestWithBucketID("POST", endpoint, msg, endpoint, []discordgo.RequestOption{}...)
		return

	default:
		// recent-posts チャンネルは除外する
		if m.ChannelID == "1157230576143695883" {
			return
		}
		if m.GuildID == "1101128103662723163" {
			return
		}
		//if m.ChannelID == "1167766542751105044" {
		//	return
		//}
		partLogger.Infof("ChannelD: %s Channel: %s\n", m.ChannelID, channelName)
		partLogger.Infof("AuthorID: %s handle: %s NickName: %s IsBot: %t\n", m.Author.ID, handle, nickName, m.Author.Bot)

		partLogger.Infof("Content: %s\n", m.Content)
		if m.Activity != nil {
			partLogger.Infof("ActivityType: %d ActivityPartyId: %s\n", m.Activity.Type, m.Activity.PartyID)
		}
		partLogger.Infof("MessageType: %d\n", m.Type)
		partLogger.Infof("MessageId: %s\n", m.ID)
		for i, a := range m.Attachments {
			partLogger.Infof("Attachements[%d]: ID: %s URL: %s Ephemeral: %t ContentType: %s Filename: %s Size: %d, Height: %d Width: %d\n", i, a.ID, a.URL, a.Ephemeral, a.ContentType, a.Filename, a.Size, a.Height, a.Width)
		}
		for i, c := range m.Components {
			j, err := c.MarshalJSON()
			if err != nil {
				partLogger.Infof("Error: %s\n", err)
			} else {
				partLogger.Infof("Components[%d]: Type: %d Json: %s\n", i, c.Type(), string(j))
			}
		}
		if m.EditedTimestamp != nil {
			partLogger.Infof("EditedTimestamp: %s\n", *m.EditedTimestamp)
		}
		for i, e := range m.Embeds {
			partLogger.Infof("Embeds[%d]: Type: %s URL: %s Description: %s Color: %d Title: %s, Timestamp: %s\n", i, e.Type, e.URL, e.Description, e.Color, e.Title, e.Timestamp)
			for ii, f := range e.Fields {
				partLogger.Infof("Embeds[%d]: Fields[%d]: Name: %s Value: %s Inline: %t\n", i, ii, f.Name, f.Value, f.Inline)
			}
			if e.Image != nil {
				partLogger.Infof("Embeds[%d]: Image: URL: %s ProxyURL: %s Height: %d Width: %d\n", i, e.Image.URL, e.Image.ProxyURL, e.Image.Height, e.Image.Width)
			}
			if e.Thumbnail != nil {
				partLogger.Infof("Embeds[%d]: Thumbnail: URL: %s ProxyURL: %s Height: %d Width: %d\n", i, e.Thumbnail.URL, e.Thumbnail.ProxyURL, e.Thumbnail.Height, e.Thumbnail.Width)
			}
			if e.Video != nil {
				partLogger.Infof("Embeds[%d]: Video: URL: %s Height: %d Width: %d\n", i, e.Video.URL, e.Video.Height, e.Video.Width)
			}
			if e.Provider != nil {
				partLogger.Infof("Embeds[%d]: Provider: Name: %s URL: %s\n", i, e.Provider.Name, e.Provider.URL)
			}
		}
		if m.Interaction != nil {
			partLogger.Infof("Interaction: ID: %s Name: %s Type: %d\n", m.Interaction.ID, m.Interaction.Name, m.Interaction.Type)
			if m.Interaction.User != nil {
				partLogger.Infof("Interaction: User: ID: %s Username: %s\n", m.Interaction.User.ID, m.Interaction.User.Username)
			}
			if m.Interaction.Member != nil {
				partLogger.Infof("Interaction: Member: ID: %s Nick: %s\n", m.Interaction.Member.User.ID, m.Interaction.Member.Nick)
			}
		}
	}
	return
}

// メッセージを送信する関数
func sendMessage(logger *zap.SugaredLogger, s *discordgo.Session, channelID string, msg string) (err error) {
	partLogger := logger.Named("sendMessage")
	defer func(partLogger *zap.SugaredLogger) {
		err := partLogger.Sync()
		if err != nil {
			logger.Error(err)
		}
	}(partLogger)

	partLogger.Info("%s %s\n", channelID, msg)
	_, err = s.ChannelMessageSend(channelID, msg)
	if err != nil {
		partLogger.Error(err)
		return err
	}

	partLogger.Infof(">>> %s", msg)
	return nil
}
