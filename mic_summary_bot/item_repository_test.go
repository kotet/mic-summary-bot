package micsummarybot

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite" // SQLite3 driver for side effects
	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB initializes an in-memory SQLite database and returns an ItemRepository and a cleanup function.
func setupTestDB(t *testing.T) (*ItemRepository, func()) {
	// Use in-memory SQLite for tests.
	dbPath := ":memory:"

	repo, err := NewItemRepository(dbPath, 3) // Default retry count for tests

	require.NoError(t, err, "NewItemRepository should not return an error")
	require.NotNil(t, repo, "NewItemRepository should return a non-nil repository")

	cleanup := func() {
		err := repo.Close()
		require.NoError(t, err, "Closing repository should not return an error")
	}
	return repo, cleanup
}

func TestNewItemRepository(t *testing.T) {
	t.Run("in-memory database", func(t *testing.T) {
		repo, err := NewItemRepository(":memory:", 3) // Default retry count for tests
		require.NoError(t, err)
		require.NotNil(t, repo)
		defer repo.Close()

		// Check if table exists by querying sqlite_master
		var name string
		err = repo.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='items';").Scan(&name)
		require.NoError(t, err, "Querying for 'items' table should succeed")
		assert.Equal(t, "items", name, "Table 'items' should exist")

		// Check if indexes exist
		var indexName string
		err = repo.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_items_url';").Scan(&indexName)
		require.NoError(t, err, "Querying for 'idx_items_url' index should succeed")
		assert.Equal(t, "idx_items_url", indexName, "Index 'idx_items_url' should exist")

		err = repo.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_items_status_published_at';").Scan(&indexName)
		require.NoError(t, err, "Querying for 'idx_items_status_published_at' index should succeed")
		assert.Equal(t, "idx_items_status_published_at", indexName, "Index 'idx_items_status_published_at' should exist")

		err = repo.db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name='idx_items_status_last_checked_at';").Scan(&indexName)
		require.NoError(t, err, "Querying for 'idx_items_status_last_checked_at' index should succeed")
		assert.Equal(t, "idx_items_status_last_checked_at", indexName, "Index 'idx_items_status_last_checked_at' should exist")
	})

	t.Run("file-based database", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "testdb_new")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		dbPath := filepath.Join(tempDir, "test.db")
		repoFile, err := NewItemRepository(dbPath, 3) // Default retry count for tests
		require.NoError(t, err)
		require.NotNil(t, repoFile)

		_, err = os.Stat(dbPath)
		assert.NoError(t, err, "DB file should be created at the specified path")

		err = repoFile.Close()
		assert.NoError(t, err, "Closing file-based repository should not fail")
	})
}

func TestItemRepository_Close(t *testing.T) {
	repo, _ := setupTestDB(t) // We don't need the cleanup func from setup as we're testing Close.

	err := repo.Close()
	assert.NoError(t, err, "Close should not return an error")

	// Verify the database connection is actually closed.
	// Further operations should fail.
	_, err = repo.db.Exec("SELECT 1")
	assert.Error(t, err, "Database operations should fail after Close")
	assert.EqualError(t, err, "sql: database is closed", "Error should indicate database is closed")
}

func TestItemRepository_insert(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().Truncate(time.Second).UTC() // Use UTC and truncate for consistency
	item := &Item{
		URL:           "http://example.com/article1",
		Title:         "Article 1",
		PublishedAt:   now,
		Status:        StatusUnprocessed,
		Reason:        ReasonNone,
		RetryCount:    0,
		CreatedAt:     now,
		LastCheckedAt: now,
	}

	err := repo.insert(context.Background(), item)
	require.NoError(t, err, "Insert should succeed for a new item")

	// Verify by querying
	var dbItem Item
	row := repo.db.QueryRow("SELECT id, url, title, published_at, status, reason, retry_count, created_at, last_checked_at FROM items WHERE url = ?", item.URL)
	err = row.Scan(&dbItem.ID, &dbItem.URL, &dbItem.Title, &dbItem.PublishedAt, &dbItem.Status, &dbItem.Reason, &dbItem.RetryCount, &dbItem.CreatedAt, &dbItem.LastCheckedAt)
	require.NoError(t, err, "Scanning inserted item should succeed")

	assert.True(t, dbItem.ID > 0, "ID should be populated")
	assert.Equal(t, item.URL, dbItem.URL)
	assert.Equal(t, item.Title, dbItem.Title)
	assert.True(t, item.PublishedAt.Equal(dbItem.PublishedAt.UTC()), "PublishedAt mismatch. Expected: %v, Got: %v", item.PublishedAt, dbItem.PublishedAt.UTC())
	assert.Equal(t, item.Status, dbItem.Status)
	assert.Equal(t, item.Reason, dbItem.Reason)
	assert.Equal(t, item.RetryCount, dbItem.RetryCount)
	assert.True(t, item.CreatedAt.Equal(dbItem.CreatedAt.UTC()), "CreatedAt mismatch. Expected: %v, Got: %v", item.CreatedAt, dbItem.CreatedAt.UTC())
	assert.True(t, item.LastCheckedAt.Equal(dbItem.LastCheckedAt.UTC()), "LastCheckedAt mismatch. Expected: %v, Got: %v", item.LastCheckedAt, dbItem.LastCheckedAt.UTC())

	// Test unique constraint on URL
	duplicateItem := &Item{
		URL:           "http://example.com/article1", // Same URL
		Title:         "Duplicate Article",
		PublishedAt:   now,
		Status:        StatusProcessed,
		Reason:        ReasonNone,
		RetryCount:    0,
		CreatedAt:     now,
		LastCheckedAt: now,
	}
	err = repo.insert(context.Background(), duplicateItem)
	require.Error(t, err, "Insert should fail for duplicate URL due to UNIQUE constraint")
	assert.Contains(t, err.Error(), "UNIQUE constraint failed: items.url", "Error message should indicate unique constraint violation")
}

func TestItemRepository_IsURLExists(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	url1 := "http://example.com/exists"
	url2 := "http://example.com/notexists"

	// Insert an item to make url1 exist
	item := &Item{URL: url1, Title: "Test", PublishedAt: time.Now(), Status: StatusUnprocessed, Reason: ReasonNone, RetryCount: 0, CreatedAt: time.Now(), LastCheckedAt: time.Now()}
	err := repo.insert(context.Background(), item)
	require.NoError(t, err)

	exists, err := repo.IsURLExists(context.Background(), url1)
	require.NoError(t, err)
	assert.True(t, exists, "IsURLExists should return true for an existing URL")

	exists, err = repo.IsURLExists(context.Background(), url2)
	require.NoError(t, err)
	assert.False(t, exists, "IsURLExists should return false for a non-existing URL")
}

// getItemByUrl is a test helper function to retrieve an item for verification.
func getItemByUrl(t *testing.T, db *sql.DB, url string) *Item {
	t.Helper()
	query := `SELECT id, url, title, published_at, status, reason, retry_count, created_at, last_checked_at FROM items WHERE url = ?`
	row := db.QueryRow(query, url)
	var item Item
	err := row.Scan(&item.ID, &item.URL, &item.Title, &item.PublishedAt, &item.Status, &item.Reason, &item.RetryCount, &item.CreatedAt, &item.LastCheckedAt)
	if err == sql.ErrNoRows {
		return nil // Item not found
	}
	require.NoError(t, err, "Failed to scan item from DB")
	item.PublishedAt = item.PublishedAt.UTC() // Ensure UTC for comparison
	item.CreatedAt = item.CreatedAt.UTC()     // Ensure UTC for comparison
	item.LastCheckedAt = item.LastCheckedAt.UTC()
	return &item
}

func TestItemRepository_AddItem(t *testing.T) {
	makeGofeedItem := func(link, title string, published time.Time) *gofeed.Item {
		// Ensure published time is UTC for consistency with DB storage and comparison
		publishedUTC := published.UTC()
		return &gofeed.Item{
			Link:            link,
			Title:           title,
			Published:       publishedUTC.Format(time.RFC3339), // gofeed uses this field
			PublishedParsed: &publishedUTC,
		}
	}

	baseTime := time.Now().Truncate(time.Second).UTC()
	timePast := baseTime.Add(-2 * time.Hour)
	timeMid := baseTime.Add(-1 * time.Hour)
	timeFuture := baseTime

	t.Run("add items to empty database", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		itemsToAdd := []*gofeed.Item{
			makeGofeedItem("http://example.com/item1", "Item 1", timePast),
			makeGofeedItem("http://example.com/item2", "Item 2", timeMid),
		}

		_, err := repo.AddItems(context.Background(), itemsToAdd)
		require.NoError(t, err)

		dbItem1 := getItemByUrl(t, repo.db, "http://example.com/item1")
		require.NotNil(t, dbItem1)
		assert.Equal(t, StatusUnprocessed, dbItem1.Status)
		assert.True(t, timePast.Equal(dbItem1.PublishedAt), "Expected %v, got %v", timePast, dbItem1.PublishedAt)

		dbItem2 := getItemByUrl(t, repo.db, "http://example.com/item2")
		require.NotNil(t, dbItem2)
		assert.Equal(t, StatusUnprocessed, dbItem2.Status)
		assert.True(t, timeMid.Equal(dbItem2.PublishedAt), "Expected %v, got %v", timeMid, dbItem2.PublishedAt)

		var count int
		err = repo.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("add items with existing, older, and newer items", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		// Pre-populate DB with an item that will be the "last published"
		existingItem := &Item{
			URL:           "http://example.com/existing",
			Title:         "Existing Article",
			PublishedAt:   timeMid, // This will be our lastPublishedAt
			Status:        StatusProcessed,
			Reason:        ReasonNone,
			RetryCount:    0,
			CreatedAt:     time.Now().UTC(),
			LastCheckedAt: time.Now().UTC(),
		}
		err := repo.insert(context.Background(), existingItem)
		require.NoError(t, err)

		itemsToAdd := []*gofeed.Item{
			makeGofeedItem("http://example.com/existing", "Existing Article Attempt", timeMid), // Duplicate URL
			makeGofeedItem("http://example.com/older", "Older Article", timePast),              // Older than timeMid
			makeGofeedItem("http://example.com/newer", "Newer Article", timeFuture),            // Newer than timeMid
		}

		_, err = repo.AddItems(context.Background(), itemsToAdd)
		require.NoError(t, err)

		// Verify duplicate was skipped (original item should remain unchanged)
		dbExisting := getItemByUrl(t, repo.db, "http://example.com/existing")
		require.NotNil(t, dbExisting)
		assert.Equal(t, "Existing Article", dbExisting.Title) // Original title
		assert.Equal(t, StatusProcessed, dbExisting.Status)   // Original status

		// Verify older item was added as StatusProcessed
		dbOlder := getItemByUrl(t, repo.db, "http://example.com/older")
		require.NotNil(t, dbOlder)
		assert.Equal(t, StatusProcessed, dbOlder.Status, "Older item should be StatusProcessed")
		assert.True(t, timePast.Equal(dbOlder.PublishedAt))

		// Verify newer item was added as StatusUnprocessed
		dbNewer := getItemByUrl(t, repo.db, "http://example.com/newer")
		require.NotNil(t, dbNewer)
		assert.Equal(t, StatusUnprocessed, dbNewer.Status, "Newer item should be StatusUnprocessed")
		assert.True(t, timeFuture.Equal(dbNewer.PublishedAt))

		var count int
		err = repo.db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count, "Total items should be 3 (1 existing + 2 new)")
	})
	t.Run("AddItem handles error from querying last published_at", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		itemsToAdd := []*gofeed.Item{
			makeGofeedItem("http://example.com/query_fail", "Query Fail", time.Now().UTC()),
		}

		// Close DB to make the initial query for last published_at fail
		repo.Close()

		_, err := repo.AddItems(context.Background(), itemsToAdd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get last published_at")
	})
}

func TestItemRepository_Update(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().Truncate(time.Second).UTC()
	initialItem := &Item{
		URL:           "http://example.com/updatable",
		Title:         "Updatable Item",
		PublishedAt:   now,
		Status:        StatusUnprocessed,
		Reason:        ReasonNone,
		RetryCount:    0,
		CreatedAt:     now,
		LastCheckedAt: now,
	}

	// Insert the initial item
	err := repo.insert(context.Background(), initialItem)
	require.NoError(t, err, "Failed to insert initial item for update test")

	// Retrieve the inserted item to get its ID
	insertedItem, err := repo.GetItemByURL(context.Background(), initialItem.URL)
	require.NoError(t, err, "Failed to retrieve inserted item for update test")
	require.NotNil(t, insertedItem, "Inserted item should not be nil")

	// Modify the item
	insertedItem.Status = StatusProcessed
	insertedItem.Reason = ReasonGeminiNotValuable
	insertedItem.RetryCount = 1

	// Record time before update
	beforeUpdate := time.Now().UTC()

	err = repo.Update(context.Background(), insertedItem)
	require.NoError(t, err, "Update should succeed")

	// Verify the update
	updatedItem, err := repo.GetItemByURL(context.Background(), initialItem.URL)
	require.NoError(t, err, "Failed to retrieve updated item")
	require.NotNil(t, updatedItem, "Updated item should not be nil")

	assert.Equal(t, insertedItem.ID, updatedItem.ID)
	assert.Equal(t, StatusProcessed, updatedItem.Status)
	assert.Equal(t, ReasonGeminiNotValuable, updatedItem.Reason)
	assert.Equal(t, 1, updatedItem.RetryCount)
	assert.Equal(t, initialItem.URL, updatedItem.URL) // Ensure other fields are not changed
	assert.Equal(t, initialItem.Title, updatedItem.Title)
	assert.True(t, initialItem.PublishedAt.Equal(updatedItem.PublishedAt.UTC()))
	assert.True(t, initialItem.CreatedAt.Equal(updatedItem.CreatedAt.UTC()))

	// Check that LastCheckedAt was updated
	assert.True(t, updatedItem.LastCheckedAt.After(beforeUpdate) || updatedItem.LastCheckedAt.Equal(beforeUpdate),
		"LastCheckedAt should be updated. Got: %v, BeforeUpdate: %v", updatedItem.LastCheckedAt, beforeUpdate)

	// Test updating a non-existent item (should not error, but affect 0 rows)
	nonExistentItem := &Item{
		ID:         99999, // Non-existent ID
		URL:        "http://example.com/nonexistent",
		Status:     StatusProcessed,
		Reason:     ReasonNone,
		RetryCount: 0,
	}
	err = repo.Update(context.Background(), nonExistentItem)
	require.NoError(t, err, "Updating a non-existent item should not error")
}

func TestItemRepository_GetItemByURL(t *testing.T) {
	repo, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now().Truncate(time.Second).UTC()
	itemToInsert := &Item{
		URL:           "http://example.com/getbyurl",
		Title:         "Get By URL Test",
		PublishedAt:   now,
		Status:        StatusUnprocessed,
		Reason:        ReasonNone,
		RetryCount:    0,
		CreatedAt:     now,
		LastCheckedAt: now,
	}
	err := repo.insert(context.Background(), itemToInsert)
	require.NoError(t, err)

	t.Run("item exists", func(t *testing.T) {
		retrievedItem, err := repo.GetItemByURL(context.Background(), "http://example.com/getbyurl")
		require.NoError(t, err)
		require.NotNil(t, retrievedItem)
		assert.Equal(t, itemToInsert.URL, retrievedItem.URL)
		assert.Equal(t, itemToInsert.Title, retrievedItem.Title)
		assert.True(t, itemToInsert.PublishedAt.Equal(retrievedItem.PublishedAt.UTC()))
		assert.Equal(t, itemToInsert.Status, retrievedItem.Status)
		assert.Equal(t, itemToInsert.Reason, retrievedItem.Reason)
		assert.Equal(t, itemToInsert.RetryCount, retrievedItem.RetryCount)
		assert.True(t, itemToInsert.CreatedAt.Equal(retrievedItem.CreatedAt.UTC()))
		assert.True(t, itemToInsert.LastCheckedAt.Equal(retrievedItem.LastCheckedAt.UTC()))
		assert.True(t, retrievedItem.ID > 0)
	})

	t.Run("item does not exist", func(t *testing.T) {
		retrievedItem, err := repo.GetItemByURL(context.Background(), "http://example.com/nonexistenturl")
		require.NoError(t, err)
		assert.Nil(t, retrievedItem)
	})
}

func TestItemRepository_GetItemForSummarization(t *testing.T) {
	baseTime := time.Now().Truncate(time.Second).UTC()

	createTestItem := func(url string, status ItemStatus, publishedAt time.Time, lastCheckedAt time.Time, retryCount int, titleSuffix string) *Item {
		return &Item{
			URL:           url,
			Title:         "Test Item " + titleSuffix,
			PublishedAt:   publishedAt,
			Status:        status,
			Reason:        ReasonNone,
			RetryCount:    retryCount,
			CreatedAt:     baseTime,
			LastCheckedAt: lastCheckedAt,
		}
	}

	t.Run("no pending items", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		item, err := repo.GetItemForSummarization(context.Background())
		require.NoError(t, err)
		assert.Nil(t, item)
	})

	t.Run("one pending item exists", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		pendingItem := createTestItem("http://example.com/pending", StatusPending, baseTime, baseTime, 0, "P")
		err := repo.insert(context.Background(), pendingItem)
		require.NoError(t, err)

		item, err := repo.GetItemForSummarization(context.Background())
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, pendingItem.URL, item.URL)
	})

	t.Run("multiple pending items exist, returns oldest published", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		itemNewer := createTestItem("http://example.com/pendingNew", StatusPending, baseTime.Add(-1*time.Hour), baseTime, 0, "PNew")
		itemOlder := createTestItem("http://example.com/pendingOld", StatusPending, baseTime.Add(-2*time.Hour), baseTime, 0, "POld")
		err := repo.insert(context.Background(), itemNewer)
		require.NoError(t, err)
		err = repo.insert(context.Background(), itemOlder)
		require.NoError(t, err)

		item, err := repo.GetItemForSummarization(context.Background())
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, itemOlder.URL, item.URL)
	})

	t.Run("last_checked_at is updated when item is retrieved", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		oldTime := baseTime.Add(-1 * time.Hour)
		pendingItem := createTestItem("http://example.com/pending", StatusPending, baseTime, oldTime, 0, "P")
		err := repo.insert(context.Background(), pendingItem)
		require.NoError(t, err)

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		item, err := repo.GetItemForSummarization(context.Background())
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, pendingItem.URL, item.URL)
		// last_checked_at should be updated and newer than the old time
		assert.True(t, item.LastCheckedAt.After(oldTime), "last_checked_at should be updated")

		// Verify in database
		dbItem, err := repo.GetItemByURL(context.Background(), item.URL)
		require.NoError(t, err)
		assert.True(t, dbItem.LastCheckedAt.After(oldTime), "last_checked_at should be updated in database")
	})
}

func TestItemRepository_GetItemForScreening(t *testing.T) {
	baseTime := time.Now().Truncate(time.Second).UTC()

	createTestItem := func(url string, status ItemStatus, publishedAt time.Time, lastCheckedAt time.Time, retryCount int, titleSuffix string) *Item {
		return &Item{
			URL:           url,
			Title:         "Test Item " + titleSuffix,
			PublishedAt:   publishedAt,
			Status:        status,
			Reason:        ReasonNone,
			RetryCount:    retryCount,
			CreatedAt:     baseTime,
			LastCheckedAt: lastCheckedAt,
		}
	}

	t.Run("no items to screen", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		item, err := repo.GetItemForScreening(context.Background())
		require.NoError(t, err)
		assert.Nil(t, item)
	})

	t.Run("returns unprocessed item first", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		unprocessed := createTestItem("http://example.com/unprocessed", StatusUnprocessed, baseTime.Add(-2*time.Hour), baseTime, 0, "U")
		deferred := createTestItem("http://example.com/deferred", StatusDeferred, baseTime.Add(-3*time.Hour), baseTime.Add(-3*time.Hour), 0, "D")
		err := repo.insert(context.Background(), unprocessed)
		require.NoError(t, err)
		err = repo.insert(context.Background(), deferred)
		require.NoError(t, err)

		item, err := repo.GetItemForScreening(context.Background())
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, unprocessed.URL, item.URL)
	})

	t.Run("returns oldest deferred if no unprocessed", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		deferredNewer := createTestItem("http://example.com/deferredNew", StatusDeferred, baseTime, baseTime.Add(-1*time.Hour), 0, "DNew")
		deferredOlder := createTestItem("http://example.com/deferredOld", StatusDeferred, baseTime, baseTime.Add(-2*time.Hour), 0, "DOld")
		pending := createTestItem("http://example.com/pending", StatusPending, baseTime, baseTime, 0, "P")
		err := repo.insert(context.Background(), deferredNewer)
		require.NoError(t, err)
		err = repo.insert(context.Background(), deferredOlder)
		require.NoError(t, err)
		err = repo.insert(context.Background(), pending)
		require.NoError(t, err)

		item, err := repo.GetItemForScreening(context.Background())
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, deferredOlder.URL, item.URL)
	})

	t.Run("last_checked_at is updated when item is retrieved", func(t *testing.T) {
		repo, cleanup := setupTestDB(t)
		defer cleanup()

		oldTime := baseTime.Add(-2 * time.Hour)
		unprocessedItem := createTestItem("http://example.com/unprocessed", StatusUnprocessed, baseTime, oldTime, 0, "U")
		err := repo.insert(context.Background(), unprocessedItem)
		require.NoError(t, err)

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		item, err := repo.GetItemForScreening(context.Background())
		require.NoError(t, err)
		require.NotNil(t, item)
		assert.Equal(t, unprocessedItem.URL, item.URL)
		// last_checked_at should be updated and newer than the old time
		assert.True(t, item.LastCheckedAt.After(oldTime), "last_checked_at should be updated")

		// Verify in database
		dbItem, err := repo.GetItemByURL(context.Background(), item.URL)
		require.NoError(t, err)
		assert.True(t, dbItem.LastCheckedAt.After(oldTime), "last_checked_at should be updated in database")
	})
}
