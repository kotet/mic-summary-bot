package micsummarybot

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// dummySizeFetcher はテスト用に常に固定のサイズを返すモック関数です。
func dummySizeFetcher(docURL string) (int64, error) {
	// テスト用にダミーのサイズを返す
	return 12345, nil
}

func TestParseHTMLForDocuments_NoPDF(t *testing.T) {
	htmlFilePath := filepath.Join("..", "resources", "example_no_pdf.htm")
	htmlContent, err := os.ReadFile(htmlFilePath)
	assert.NoError(t, err)

	baseURL, err := url.Parse("https://www.soumu.go.jp/menu_news/s-news/01toukei07_01000272.html")
	assert.NoError(t, err)

	documents, err := parseHTMLForDocuments(string(htmlContent), baseURL, dummySizeFetcher)
	assert.NoError(t, err)
	assert.Empty(t, documents, "No PDF documents should be found")
}

func TestParseHTMLForDocuments_OnlyPDF(t *testing.T) {
	htmlFilePath := filepath.Join("..", "resources", "example_only_pdf.htm")
	htmlContent, err := os.ReadFile(htmlFilePath)
	assert.NoError(t, err)

	baseURL, err := url.Parse("https://www.soumu.go.jp/main_sosiki/joho_tsusin/policyreports/joho_tsusin/yusei/yusei_gyousei/02ryutsu01_04000470.html")
	assert.NoError(t, err)

	documents, err := parseHTMLForDocuments(string(htmlContent), baseURL, dummySizeFetcher)
	assert.NoError(t, err)
	assert.Len(t, documents, 4, "Should find 4 PDF documents")

	expectedURLs := []string{
		"https://www.soumu.go.jp/main_content/001014570.pdf",
		"https://www.soumu.go.jp/main_content/001014571.pdf",
		"https://www.soumu.go.jp/main_content/001014572.pdf",
		"https://www.soumu.go.jp/main_content/001014943.pdf",
	}
	for i, doc := range documents {
		assert.Equal(t, expectedURLs[i], doc.URL)
		assert.Equal(t, int64(12345), doc.Size) // dummySizeFetcherが返すサイズ
	}
}

func TestParseHTMLForDocuments_WithNonPDF(t *testing.T) {
	htmlFilePath := filepath.Join("..", "resources", "example_with_non_pdf.htm")
	htmlContent, err := os.ReadFile(htmlFilePath)
	assert.NoError(t, err)

	baseURL, err := url.Parse("https://www.soumu.go.jp/menu_news/s-news/01kiban04_02000258.html")
	assert.NoError(t, err)

	documents, err := parseHTMLForDocuments(string(htmlContent), baseURL, dummySizeFetcher)
	assert.NoError(t, err)
	assert.Len(t, documents, 1, "Should find 1 PDF document")

	expectedURLs := []string{
		"https://www.soumu.go.jp/main_content/001014454.pdf",
	}
	for i, doc := range documents {
		assert.Equal(t, expectedURLs[i], doc.URL)
		assert.Equal(t, int64(12345), doc.Size) // dummySizeFetcherが返すサイズ
	}
}
