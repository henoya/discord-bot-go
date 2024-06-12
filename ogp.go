package main

import (
	"golang.org/x/net/html"
	"io"
	"net/http"
)

// OGPData OGPデータを格納するための構造体
type OGPData struct {
	Title       string
	Type        string
	Image       string
	URL         string
	Description string
}

// 指定されたURLからHTMLを取得する関数
func fetchHTML(url string) (*html.Node, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// OGPデータを抽出する関数
func extractOGP(doc *html.Node) *OGPData {
	var ogpData OGPData
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "meta" {
			var property, content string
			for _, attr := range node.Attr {
				if attr.Key == "property" {
					property = attr.Val
				} else if attr.Key == "content" {
					content = attr.Val
				}
			}
			switch property {
			case "og:title":
				ogpData.Title = content
			case "og:type":
				ogpData.Type = content
			case "og:image":
				ogpData.Image = content
			case "og:url":
				ogpData.URL = content
			case "og:description":
				ogpData.Description = content
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			crawler(c)
		}
	}
	crawler(doc)
	return &ogpData
}
