### DBテーブル設計書

# データベース設計書：Botアイテム状態管理

## 1. 概要

総務省RSS→Gemini要約→Mastodon自動投稿Botの、RSSアイテムの状態管理に使用するSQLite3データベースの設計を定義する。glebarez/go-sqliteライブラリを利用する。

## 2. テーブル定義

### 2.1 `items` テーブル

RSSフィードから取得される各アイテムの状態と関連情報を管理する。

* **テーブル名**: `items`

* **目的**: RSSアイテムの重複排除、処理状況の管理、および優先度制御を行う。

* **カラム**

| カラム名       | 型        | 制約                       | 説明                                                                                                                                            |
| :------------- | :-------- | :------------------------- | :---------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`           | INTEGER   | PRIMARY KEY, AUTOINCREMENT | レコードのユニークID                                                                                                                            |
| `url`          | TEXT      | NOT NULL, UNIQUE           | RSSアイテムのリンクURL。重複排除のキーとして使用される                                                                                          |
| `title`        | TEXT      | NOT NULL                   | RSSアイテムのタイトル                                                                                                                           |
| `published_at` | TIMESTAMP | NOT NULL                   | RSSアイテムの発行日時                                                                                                                           |
| `status`       | INTEGER   | NOT NULL                   | アイテムの処理状態。`0`: unprocessed, `1`: deferred, `2`: pending, `3`: processed のいずれか。                                               |
| `reason`       | INTEGER   | NULLABLE                   | `status`が `1` (deferred) または `3` (processed) になった場合の理由コード。                                                                    |
| `retry_count`  | INTEGER   | NOT NULL                   | 処理を試行したがエラーまたはスキップにより失敗した回数。一定回数以上の失敗が続いた場合、`status`を`processed`に変更する                         |
| `created_at`      | TIMESTAMP | NOT NULL                   | レコードが作成された日時。                                                                                                                      |
| `last_checked_at` | TIMESTAMP | NOT NULL                   | アイテムが最後に処理対象としてチェックされた日時。                                                                                              |

* **インデックス**

    * `idx_items_url`: `url`カラムに対するユニークインデックス。重複排除の高速化のため。
    * `idx_items_status_last_checked_at`: `status`と`last_checked_at`カラムに対するインデックス。未処理アイテムおよび先送りアイテムの効率的な取得のため。
    * `idx_items_status_published_at`: `status`と`published_at`カラムに対するインデックス。処理待ちアイテムの効率的な取得のため。

## 3. 状態遷移とデータ操作

1.  **新規アイテムの追加**:
    * RSSフィードから新しいアイテムが取得された場合、`status`を`0` (`unprocessed`)、`retry_count`を`0`として`items`テーブルに追加する。
    * `last_checked_at`はレコード作成日時と同じ値を設定する。
    * `url`が既存のレコードと重複する場合、新規追加は行わない。

2.  **アイテムの選択**:
    * **要約処理**: `status`が`2` (`pending`) のアイテムの中から`published_at`が最も古いものを1件選択し、処理を試みる。
    * **スクリーニング処理**: `pending`のアイテムがない場合、`status`が`0` (`unprocessed`) または `1` (`deferred`) のアイテムの中から`last_checked_at`が最も古いものを1件選択し、処理を試みる。

3.  **処理結果に応じた状態更新**:
    * `status`を更新するすべてのケースで、`last_checked_at`を現在の時刻に更新する。
    * **スクリーニング成功（要約価値あり）**:
        * `status`を`2` (`pending`) に更新する。
    * **要約・投稿成功**:
        * `status`を`3` (`processed`) に更新する。
        * `reason`を`0` (`ReasonNone`) に更新する。
    * **Geminiの判定が「価値がない」または「ページ未完成」**:
        * `status`を`1` (`deferred`) に更新する。
        * `reason`に該当する`ItemReasonCode`を記録する。
        * `retry_count`をインクリメントする。
    * **ファイルダウンロード失敗、Gemini API呼び出し失敗など**:
        * `status`を`1` (`deferred`) に更新する。
        * `reason`に該当する`ItemReasonCode`
        * `retry_count`をインクリメントする。
    * **リトライ回数上限超過**:
        * `retry_count`が設定された上限値を超えた場合、`status`を`3` (`processed`) に更新する。
        * `reason`に`6` (`ReasonRetryLimitExceeded`) を記録する。

4.  **処理済みアイテムの扱い**:
    * `status`が`3` (`processed`) のアイテムは、URLの重複排除のためにのみ使用され、それ以上の処理は行われない。

### アプリケーション側の`enum`定義例 (Go言語)

```go
package micsummary

// ItemStatus はアイテムの処理状態を表す
type ItemStatus int

const (
	StatusUnprocessed ItemStatus = iota // 0: unprocessed
	StatusDeferred                      // 1: deferred
	StatusPending                       // 2: pending
	StatusProcessed                     // 3: processed
)

// ItemReasonCode はアイテムが先送りまたは処理済みになった理由を表すコード
type ItemReasonCode int

const (
	ReasonNone                 ItemReasonCode = iota() // 理由なし
	ReasonGeminiNotValuable    ItemReasonCode // Gemini判定: 要約する価値なし
	ReasonGeminiPageNotReady   ItemReasonCode // Gemini判定: ページがまだ完成していない
	ReasonDownloadFailed       ItemReasonCode // ファイルダウンロード失敗
	ReasonLargeFileSkipped     ItemReasonCode // ファイルサイズが大きすぎるため要約スキップ
	ReasonAPIFailed            ItemReasonCode // Gemini/Mastodon API呼び出し失敗
	ReasonRetryLimitExceeded   ItemReasonCode // リトライ回数上限超過
)
```