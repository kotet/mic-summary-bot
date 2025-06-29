package micsummarybot

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite" // SQLite3 driver
	"github.com/mmcdole/gofeed"
)

// ItemStatus はアイテムの処理状態を表す
type ItemStatus int

const (
	StatusUnprocessed ItemStatus = iota // 0: unprocessed（未処理）
	StatusDeferred                      // 1: deferred（先送り）
	StatusProcessed                     // 2: processed（処理済み）
)

// ItemReasonCode はアイテムが先送りまたは処理済みになった理由を表すコード
type ItemReasonCode int

const (
	ReasonNone               ItemReasonCode = iota // 0: 理由なし (通常はprocessedに遷移した場合)
	ReasonGeminiNotValuable                        // 1: Gemini判定: 要約する価値なし
	ReasonGeminiPageNotReady                       // 2: Gemini判定: ページがまだ完成していない
	ReasonDownloadFailed                           // 3: ファイルダウンロード失敗
	ReasonLargeFileSkipped                         // 4: ファイルサイズが大きすぎるため要約スキップ
	ReasonAPIFailed                                // 5: Gemini/Mastodon API呼び出し失敗
	ReasonRetryLimitExceeded                       // 6: リトライ回数上限超過
)

// Item は items テーブルのレコードを表す構造体
type Item struct {
	ID          int
	URL         string
	Title       string
	PublishedAt time.Time
	Status      ItemStatus
	Reason      ItemReasonCode
	RetryCount  int
	CreatedAt   time.Time
}

// ItemRepository は items テーブルへの操作を提供する
type ItemRepository struct {
	db                    *sql.DB
	maxDeferredRetryCount int
}

// formatQuery
func formatQuery(query string) string {
	query = strings.ReplaceAll(query, "\n", " ")
	query = strings.TrimSpace(query)
	return query
}

// NewItemRepository は新しいItemRepositoryインスタンスを作成し、データベース接続を初期化します。
// テーブルが存在しない場合は作成します。
func NewItemRepository(dbPath string, maxDeferredRetryCount int) (*ItemRepository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify the database connection
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to verify database connection: %w", err)
	}

	// テーブル作成
	createTableSQLs := []string{
		`CREATE TABLE IF NOT EXISTS items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL UNIQUE,
		title TEXT NOT NULL,
		published_at TIMESTAMP NOT NULL,
		status INTEGER NOT NULL,
		reason INTEGER NOT NULL,
		retry_count INTEGER NOT NULL,
		created_at TIMESTAMP NOT NULL
	);`,
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_items_url ON items(url);",
		"CREATE INDEX IF NOT EXISTS idx_items_status_published_at ON items(status, published_at);",
	}
	for _, createTableSQL := range createTableSQLs {
		_, err = db.Exec(formatQuery(createTableSQL))
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return &ItemRepository{db: db, maxDeferredRetryCount: maxDeferredRetryCount}, nil
}

// Close はデータベース接続を閉じます。
func (r *ItemRepository) Close() error {
	return r.db.Close()
}

func withTransaction(ctx context.Context, db *sql.DB, txFunc func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		pkgLogger.Error("transaction error", "error", err)
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			pkgLogger.Error("transaction rollback", "error", err)
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()
	err = txFunc(tx)
	return err
}

// insert
func (r *ItemRepository) insert(ctx context.Context, item *Item) error {
	insertSQL := `
	INSERT INTO items (url, title, published_at, status, reason, retry_count, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	err := withTransaction(ctx, r.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(insertSQL, item.URL, item.Title, item.PublishedAt, item.Status, item.Reason, item.RetryCount, item.CreatedAt)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to insert item: %w", err)
	}
	return nil
}

// Update
func (r *ItemRepository) Update(ctx context.Context, item *Item) error {
	updateSQL := `
	UPDATE items
	SET status = ?, reason = ?, retry_count = ?
	WHERE id = ?;
	`
	err := withTransaction(ctx, r.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(updateSQL, item.Status, item.Reason, item.RetryCount, item.ID)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update item ID %d: %w", item.ID, err)
	}
	return nil
}

// GetItemByURL
func (r *ItemRepository) GetItemByURL(ctx context.Context, url string) (*Item, error) {
	query := formatQuery(`
	SELECT id, url, title, published_at, status, reason, retry_count, created_at
	FROM items
	WHERE url = ?;
	`)

	row := r.db.QueryRowContext(ctx, query, url)
	var item Item
	err := row.Scan(&item.ID, &item.URL, &item.Title, &item.PublishedAt, &item.Status, &item.Reason, &item.RetryCount, &item.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // Item not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get item by URL %s: %w", url, err)
	}
	return &item, nil
}

// GetUnprocessedItems
func (r *ItemRepository) GetUnprocessedItems(ctx context.Context) ([]*Item, error) {
	query := formatQuery(`
	SELECT id, url, title, published_at, status, reason, retry_count, created_at
	FROM items
	WHERE status = ?
	ORDER BY published_at ASC
	LIMIT 1;
	`)
	rows, err := r.db.QueryContext(ctx, query, StatusUnprocessed)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get unprocessed items: %w", err)
	}
	defer rows.Close()

	if err == nil && rows.Next() {
		var item Item
		err = rows.Scan(&item.ID, &item.URL, &item.Title, &item.PublishedAt, &item.Status, &item.Reason, &item.RetryCount, &item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan unprocessed item: %w", err)
		}
		// Ensure times are UTC for consistency
		item.PublishedAt = item.PublishedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		return []*Item{&item}, nil
	}

	// If no unprocessed items, check for deferred items with retry attempts left
	query = formatQuery(`
	SELECT id, url, title, published_at, status, reason, retry_count, created_at
	FROM items
	WHERE status = ? AND retry_count < ?
	ORDER BY published_at ASC
	LIMIT 1;
	`)
	rows, err = r.db.QueryContext(ctx, query, StatusDeferred, r.maxDeferredRetryCount) // Assuming max 3 retries for deferred
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get deferred items: %w", err)
	}
	defer rows.Close()

	if err == nil && rows.Next() {
		var item Item
		err = rows.Scan(&item.ID, &item.URL, &item.Title, &item.PublishedAt, &item.Status, &item.Reason, &item.RetryCount, &item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deferred item: %w", err)
		}
		// Ensure times are UTC for consistency
		item.PublishedAt = item.PublishedAt.UTC()
		item.CreatedAt = item.CreatedAt.UTC()
		return []*Item{&item}, nil
	}

	return nil, nil // No items found in either state
}

// IsURLExists
func (r *ItemRepository) IsURLExists(ctx context.Context, url string) (bool, error) {
	query := `SELECT COUNT(*) FROM items WHERE url = ?;`
	var count int
	err := r.db.QueryRowContext(ctx, query, url).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AddItems は新しいRSSアイテムをデータベースに追加します。
// URLが既存のレコードと重複する場合、新規追加は行いません。
func (r *ItemRepository) AddItems(ctx context.Context, items []*gofeed.Item) (int, error) {
	addedCount := 0
	// 最も新しいエントリを取得
	lastPublishedAt, err := func() (*time.Time, error) {
		lastResult, err := r.db.Query("SELECT published_at FROM items ORDER BY published_at DESC LIMIT 1;")
		if err != nil {
			return nil, fmt.Errorf("failed to get last published_at: %w", err)
		}
		defer lastResult.Close()
		var lastPublishedAt *time.Time = nil
		if lastResult.Next() {
			lastPublishedAt = new(time.Time)
			lastResult.Scan(lastPublishedAt)
		}
		return lastPublishedAt, nil
	}()
	if err != nil {
		return 0, err
	}

	for _, item := range items {
		exists, err := r.IsURLExists(ctx, item.Link)
		if err != nil {
			return 0, fmt.Errorf("failed to check URL existence: %w", err)
		}
		if exists {
			// 既に存在したらスキップ
			continue
		}

		// 存在しない場合、前回の最新より古いものは処理済みとする
		if lastPublishedAt != nil && item.PublishedParsed.Before(*lastPublishedAt) {
			err = r.insert(ctx, &Item{
				URL:         item.Link,
				Title:       item.Title,
				PublishedAt: *item.PublishedParsed,
				Status:      StatusProcessed,
				Reason:      ReasonNone,
				RetryCount:  0,
				CreatedAt:   time.Now(),
			})
			if err != nil {
				return 0, err
			}
		} else {
			err = r.insert(ctx, &Item{
				URL:         item.Link,
				Title:       item.Title,
				PublishedAt: *item.PublishedParsed,
				Status:      StatusUnprocessed,
				Reason:      ReasonNone,
				RetryCount:  0,
				CreatedAt:   time.Now(),
			})
			if err != nil {
				return 0, err
			}
			addedCount++
		}
	}
	return addedCount, nil
}

func (r *ItemRepository) CountUnprocessedItems(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM items WHERE status = ?;`
	var count int
	err := r.db.QueryRowContext(ctx, query, StatusUnprocessed).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
