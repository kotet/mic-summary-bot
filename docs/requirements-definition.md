# 要件定義書：総務省RSS→Gemini要約→Mastodon自動投稿Bot

## 1. はじめに

* 作成日：2025年06月01日
* 作成者：一般市民（総務省の動向を注視し、情報を広く共有したい者）
* 開発言語：Go
* 目的：総務省が公開するホームページ新着情報（RSSフィード）から議事録等のPDF資料を自動で取得し、Gemini API（google.golang.org/genaiライブラリ）を用いて要約を生成、その要約をMastodonに定期投稿するBotをGoで開発する。
* 背景：一般市民として総務省の動きを迅速に把握し、ソーシャルメディア上で同じ関心を持つ市民に情報を自動的に共有することで、透明性の向上と迅速な情報配信を図る。

## 2. 対象範囲（Scope）

1. 総務省ホームページ新着情報RSS（[https://www.soumu.go.jp/news.rdf）の監視](https://www.soumu.go.jp/news.rdf）の監視)
2. RSSアイテムに紐づくWebページからのPDFまたはその他Geminiが直接処理可能なファイルのダウンロード
3. ダウンロードしたファイルをそのままGemini API（google.golang.org/genai）で要約生成
4. 要約内容の判定（「要約する価値がある」/「価値がない」/「まだページが完成していない」）をGeminiに依頼し、必要に応じて要約実行
5. 要約結果（または判定結果）をMastodonへ自動投稿
6. 定期実行および既読管理による重複排除

### 2.1 非対象

* 人手による手動操作での投稿UI
* PDF以外のファイル形式（Word, PowerPoint等）でGeminiが処理できない場合は要約対象外
* 過去に処理したアイテムの再投稿（既読管理機構でスキップ）

## 3. 用語定義

* **Bot**: `micsummary.Bot` 構造体およびそれが保持する処理ロジック全体（Goで実装）。
* **RSSアイテム**: RSSフィードから取得される各エントリ。
* **Gemini API**: Googleの大規模言語モデルAPI。Goでは `google.golang.org/genai` パッケージを利用。
* **Mastodon**: 分散型SNS。Botは指定インスタンスに投稿を行う
* **処理可能ファイル**: PDFなど、Geminiが直接処理可能な形式。以下のファイルが該当する:  
    * PDF - application/pdf
    * JavaScript - application/x-javascript、text/javascript
    * Python - application/x-python、text/x-python
    * TXT - text/plain
    * HTML - text/html
    * CSS - text/css
    * Markdown - text/md
    * CSV - text/csv
    * XML - text/xml
    * RTF - text/rtf

## 4. 機能要件（Functional Requirements）

### 4.1 RSSフィード取得・解析

* **機能ID: FR-001**

  * **要件**: Go言語の標準ライブラリまたは外部パッケージを用い、総務省ホームページ新着情報RSS（[https://www.soumu.go.jp/news.rdf）を取得する。](https://www.soumu.go.jp/news.rdf）を取得する。)
  * **更新間隔**: ライブラリとして実装し、関数の呼び出し頻度を調整することで更新間隔を設定可能とする。
  * **フィルタリング・判定**:

    * Goのコード内でRSSアイテムのタイトル、リンク先ページの内容サマリー情報、前回記事タイトル情報をGemini判定用プロンプトに渡し、「要約する価値がある」/「価値がない」/「まだ完成していない」の3つのステータスをGeminiに判定してもらう。
    * 判定結果が「要約する価値がある」の場合にのみ、ファイル取得～要約～投稿を行う。
    * 「価値がない」または「まだページが完成していない」の場合は処理せずに次サイクルに持ち越すかスキップする。
  * **既読管理**:
    * RSSアイテムのURLをキーに、各アイテムの状態（未処理、先送り、処理済み）をデータベースで管理。ライブラリはglebarez/go-sqliteを利用し、SQLite3データベースに保存
    * 前回取得時よりもタイムスタンプが古いアイテムは、たとえデータベースに存在しなくても処理済みとして扱う。

### 4.2 対象ページとファイル取得

* **機能ID: FR-002**

  * **要件**: Goの `net/http` パッケージを使い、RSSアイテムから得られたHTMLページのURLを取得し、ページ内のGeminiが処理可能なファイル（主にPDFリンク）を抽出する。
  * **リンク抽出ルール**:

    * Goの HTML パーサー（例: `golang.org/x/net/html`）を用いて `<a>` タグで `.pdf` 拡張子を含むURLを対象とし、複数存在する場合は最新のもの、またはサイズ・日付指定があればそれに従う。
    * HTML構造の特定クラスやIDがある場合はその属性を優先して抽出。
  * **ファイルサイズチェック**:

    * Content-Length を取得し、ファイルサイズが50MBを超える場合はファイルをダウンロードせずに判定用プロンプトに「ファイル名のみ」を渡す。
  * **ダウンロード先**:

    * `config` の `storage.download_dir` で指定された一時ディレクトリに Go の `io.Copy` とストリームを用いて保存。
    * 保存後、要約完了またはスキップ後には Go の `os.Remove` で一時ファイルを削除する（`storage.keep_local_copy` が `false` の場合）。
  * **メモリ制約対応**:

    * Raspberry Pi Zero（メモリ512MB）のため、ファイルはディスク上にストリーム保存し、一部ずつ読み込んで処理する。

### 4.3 要約生成・判定（Gemini API呼び出し）

* **機能ID: FR-003**

  * **要件**: ダウンロードしたファイル（PDFなど）をそのままGoの `google.golang.org/genai` パッケージを利用してGemini APIに送信し、要約を取得する。
  * **APIキー管理**:

    * `config`構造体に `gemini.api_key` 欄を用意し、利用者が好きな方法（環境変数読み込み等）で設定可能とする。
  * **モデル指定**:

    * `config`の `gemini.model` でモデル名を指定可能。
    * Goのインターフェイスを通じてプロンプト受け渡しインターフェイスを定義し、モデルごとのパラメータや実装を構造体としてDI（依存性注入）できるようにする。
  * **要約文字数・フォーマット**:

    * 要約の最大文字数上限は500文字を想定し、目安として250文字程度を返すよう指示。
    * 日本語で文章形式の要約を生成。
  * **判定プロンプト**:

    * Goのコード内でRSSアイテムのタイトル、前回記事のタイトル情報、ページURLやファイル名などを含む文字列をプロンプトに組み込み、「要約する価値」「価値がない」「まだページが完成していない」を判定。
  * **リトライ・タイムアウト**:

    * `config`に `gemini.retry_count`（例: 3回）、`gemini.retry_interval_sec`（例: 5秒）を設定可能。
    * API呼び出しが失敗した場合はGoのリトライ処理を行い、それでも失敗したら `log` パッケージで出力して当該アイテムをスキップ。
  * **ファイル送信失敗時**:

    * ファイルサイズが50MBを超えた場合、価値判定処理と要約処理にはファイル名と「ファイルサイズが大きすぎるため要約できない」旨を渡す。

### 4.4 Mastodon投稿

* **機能ID: FR-004**

  * **要件**: Goの `github.com/mattn/go-mastodon` ライブラリを利用して要約結果または判定結果を指定のMastodonインスタンスに投稿する。
  * **認証情報**:

    * `config`に `mastodon.instance_url`、`mastodon.access_token` を設定し、利用者が用意する。
  * **投稿フォーマット**:

    ```go
    client := mastodon.NewClient(&mastodon.Config{
      Server:       config.Mastodon.InstanceURL,
      AccessToken:  config.Mastodon.AccessToken,
    })
    status := fmt.Sprintf("%s%s %s", title, summary, url)
    _, err := client.PostStatus(context.Background(), &mastodon.Toot{ Status: status, })
    if err != nil {
      log.Printf("ERROR: Mastodon投稿失敗: %v", err) return err
    }
    ```
    - タイトルはRSSアイテムのタイトルを流用。 - 要約はGeminiから取得した文字列。 - 元URLはRSSアイテムから直接取得したHTMLページのURL。

* **文字数制限**:

  * Mastodonの制限はデフォルトで5000文字のため、文字数制限チェックロジックはBotに用意せず、投稿エラー時は当該投稿をスキップする。

### 4.5 実行スケジューリング 実行スケジューリング

* **機能ID: FR-005**

  * **要件**: exampleパッケージのmain() 関数（Go） の無限ループ内で、引数なしの `bot.Run()` を呼び出す。実行間隔は `time.Sleep(time.Minute * {interval_minutes})` を利用。デフォルトは5分。
  * **サーバー再起動時**:

    * Bot起動時に既読キャッシュを読み込み、未処理のアイテムのみ処理。

## 5. 非機能要件（Non-Functional Requirements）

1. **安定性・可用性**

   * Botがエラーで停止しないよう、回復可能なエラーは `log` パッケージで出力後に次サイクルへ移行。
   * 連続でリトライ回数以上の失敗が発生した場合、該当アイテムをスキップし、ログレベルERRORで出力。
2. **可観測性**

   * Goの `log` パッケージを使用し、INFO/ERRORレベルで標準出力に出力。
   * ログには以下のステータスを含める：RSS取得成功／失敗、リンク抽出成功／失敗、ダウンロード成功／失敗、要約生成成功／失敗、投稿成功／失敗。
   * メトリクス出力は不要。
3. **セキュリティ**

   * `config.example.yaml` を用意し、`config.yaml` は `.gitignore` に追加。
   * .aiignoreも認証情報がアップロードされないように設定する。
   * 認証情報を含まない `config.example.yaml` をコピーして使用。
   * PDF取得時に認証が必要となった場合、エラーとして扱い、判定結果に失敗理由を含めて投稿する。
   * 外部ライブラリのバージョン管理・脆弱性チェックは利用側に委譲。
4. **性能・スケーラビリティ**

   * Raspberry Pi Zero（メモリ512MB、シングルスレッド）の制約内で動作する設計。
   * 1回の `Run` では1件の記事を処理。
   * 並列化は行わず、シンプルに直列処理。
5. **運用・保守性**

   * 設定変更は `config.yaml` のみで行い、コード変更不要。
   * Botはライブラリとして提供されるため、他プログラムから `micsummary.NewBot(config)` を呼び出して利用可能。
   * テスト用途として、URLではなくローカルファイルから `ページ内容` 構造体を生成できるモードを提供。

## 6. システム構成イメージ

```text
+----------------------+        +------------------+        +-------------------------+
|                      |  RSS   |                  |  ファイル  |                         |
| 総務省 RSS           +------->+ micsummary.Bot   +-------->+ 一時／永続ストレージ      |
| (https://.../news.rdf)|取得   | (Go)             | ダウンロード| (Raspberry Pi ZeroのSD)   |
|                      |        |                  | 保存     |                         |
+----------------------+        +---------+--------+        +-----------+-------------+
                                             |
                                             | Gemini API (google.golang.org/genai) 要約生成
                                             v
                                        +----+------------+
                                        |    Gemini API   |
                                        +----+------------+
                                             |
                                             | 要約または判定結果
                                             v
                                        +----+------------+
                                        |   Mastodon      |
                                        |   投稿API       |
                                        +-----------------+
```

## 7. 設計・実装方針

### 7.1 パッケージ構成（例）

```text
example/
└─ main.go
micsummary/
├─ config/                // 設定読み込み関連
│   └─ config.go
├─ rss/                   // RSS取得・パース関連
│   └─ rss_client.go
├─ downloader/            // ファイルダウンロード関連
│   └─ file_downloader.go
├─ summarizer/            // Gemini API呼び出し関連
│   └─ summarizer.go      // google.golang.org/genai を用いる
├─ mastodon/              // Mastodon投稿関連
│   └─ mastodon_client.go
├─ bot.go                 // Bot本体（NewBot, Run, ProcessItem メソッドなど）
├─ errors.go              // エラー定義
└─ utils.go               // ログ・ユーティリティなど
```

### 7.2 Config構造体（Go）

```go
// Config は Bot の設定情報を保持する
type Config struct {
  RSS struct {
    URL             string   `yaml:"url"`              // RSSフィードURL
  } `yaml:"rss"`

  Gemini struct {
    APIKey           string `yaml:"api_key"`           // Gemini APIキー
    Model            string `yaml:"model"`             // 利用モデル名
    MaxTokens        int    `yaml:"max_tokens"`        // 要約最大トークン数
    SummaryStyle     string `yaml:"summary_style"`     // "paragraph"
    RetryCount       int    `yaml:"retry_count"`       // リトライ回数
    RetryIntervalSec int    `yaml:"retry_interval_sec"`// リトライ間隔（秒）
  } `yaml:"gemini"`

  Mastodon struct {
    InstanceURL   string   `yaml:"instance_url"`   // MastodonインスタンスURL
    AccessToken   string   `yaml:"access_token"`   // アクセストークン
  }

  Storage struct {
    DownloadDir     string `yaml:"download_dir"`     // 一時ファイル保存ディレクトリ
    KeepLocalCopy   bool   `yaml:"keep_local_copy"`  // 要約後のローカルコピー保持フラグ
  } `yaml:"storage"`
}
```
