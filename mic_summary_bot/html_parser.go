package micsummarybot

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/japanese"
)

// GetHTMLSummary は指定されたURLからHTMLを取得し、パースしてHTMLSummary構造体を返します。
func GetHTMLSummary(targetURL string) (*HTMLandDocuments, error) {
	resp, err := http.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTML from %s: %w", targetURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get HTML from %s: status code %d", targetURL, resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	// read as Shift-JIS
	shiftJISReader := japanese.ShiftJIS.NewDecoder().Reader(bytes.NewReader(bodyBytes))
	htmlBytes, err := io.ReadAll(shiftJISReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Shift-JIS to UTF-8: %w", err)
	}

	htmlContent, err := extractContentsBody(string(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to extract contentsBody: %w", err)
	}

	baseURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	// parseHTMLForDocumentsを呼び出し、getDocumentSizeを渡す
	documents, err := parseHTMLForDocuments(string(htmlContent), baseURL, getDocumentSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML and extract documents: %w", err)
	}

	return &HTMLandDocuments{
		HTMLContent: htmlContent,
		Documents:   documents,
	}, nil
}

func extractContentsBody(htmlContent string) ([]byte, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	// extract .contentsBody
	var contentsBody *html.Node

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "class" && a.Val == "contentsBody" {
					contentsBody = n
					return
				}
			}
		}
		if contentsBody == nil {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
	}

	f(doc)

	if contentsBody == nil {
		return nil, fmt.Errorf("contentsBody not found")
	}
	var buf bytes.Buffer
	w := io.Writer(&buf)
	err = html.Render(w, contentsBody)
	if err != nil {
		return nil, fmt.Errorf("failed to render contentsBody: %w", err)
	}
	return buf.Bytes(), nil
}

// parseHTMLForDocuments はHTMLコンテンツをパースし、PDFドキュメントのURLを抽出します。
// sizeFetcher はドキュメントサイズを取得するための関数です。
func parseHTMLForDocuments(htmlContent string, baseURL *url.URL, sizeFetcher func(string) (int64, error)) ([]Document, error) {
	doc, err := html.Parse(bytes.NewReader([]byte(htmlContent)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var documents []Document
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					link := a.Val
					if strings.HasSuffix(link, ".pdf") {
						resolvedURL := resolveURL(baseURL, link)
						size, err := sizeFetcher(resolvedURL) // sizeFetcherを使用
						if err != nil {
							pkgLogger.Warn("Could not get size", "url", resolvedURL, "error", err)
							size = 0 // エラー時はサイズを0とする
						}
						documents = append(documents, Document{URL: resolvedURL, Size: size})
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return documents, nil
}

// resolveURL は相対URLを絶対URLに解決します。
func resolveURL(baseURL *url.URL, relativePath string) string {
	rel, err := url.Parse(relativePath)
	if err != nil {
		return relativePath // パースできない場合はそのまま返す
	}
	return baseURL.ResolveReference(rel).String()
}

// getDocumentSize は指定されたURLのドキュメントサイズをHTTP HEADリクエストで取得します。
func getDocumentSize(docURL string) (int64, error) {
	resp, err := http.Head(docURL)
	if err != nil {
		return 0, fmt.Errorf("failed to get HEAD for %s: %w", docURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HEAD request for %s returned status code %d", docURL, resp.StatusCode)
	}

	contentLength := resp.Header.Get("Content-Length")
	if contentLength == "" {
		return 0, fmt.Errorf("Content-Length header not found for %s", docURL)
	}

	size, err := strconv.ParseInt(contentLength, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse Content-Length '%s' for %s: %w", contentLength, docURL, err)
	}

	return size, nil
}
