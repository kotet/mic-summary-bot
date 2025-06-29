package micsummarybot

// Document はHTMLドキュメント内に添付されているドキュメントの情報を保持します。
type Document struct {
	URL  string
	Size int64 // バイト単位
}

// HTMLandDocuments はHTMLコンテンツとその中に添付されているドキュメントのリストを保持します。
type HTMLandDocuments struct {
	HTMLContent []byte
	Documents   []Document
}
