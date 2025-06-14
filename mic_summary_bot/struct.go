package micsummarybot

// Document はHTMLドキュメント内に添付されているドキュメントの情報を保持します。
type Document struct {
	URL  string
	Size int64 // バイト単位
}

// HTMLSummary はHTMLコンテンツとその中に添付されているドキュメントのリストを保持します。
type HTMLSummary struct {
	HTMLContent string
	Documents   []Document
}
