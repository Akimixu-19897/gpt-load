package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	"gpt-load/internal/encryption"
	"gpt-load/internal/httpclient"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/services"
	"gpt-load/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestHandleProxySuccessResetsFailureCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok","choices":[]}`))
	}))
	defer upstream.Close()

	db := openProxyTestDB(t)
	memStore := store.NewMemoryStore()
	settingsManager := config.NewSystemSettingsManager()
	encryptionSvc, err := encryption.NewService("")
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	group := models.Group{
		Name:        "proxy-success-reset",
		GroupType:   "standard",
		ChannelType: "openai",
		TestModel:   "gpt-4.1-nano",
		Upstreams:   datatypes.JSON([]byte(`[{"url":"` + upstream.URL + `","weight":1}]`)),
		Config: datatypes.JSONMap{
			"max_retries":         0,
			"blacklist_threshold": 3,
			"key_selection_mode":  "priority",
		},
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group: %v", err)
	}

	provider := keypool.NewProvider(db, memStore, settingsManager, encryptionSvc)
	keys := []models.APIKey{{
		GroupID:      group.ID,
		KeyValue:     "key-with-previous-failure",
		KeyHash:      encryptionSvc.Hash("key-with-previous-failure"),
		Status:       models.KeyStatusActive,
		Weight:       10,
		FailureCount: 1,
	}}
	if err := provider.AddKeys(group.ID, keys); err != nil {
		t.Fatalf("AddKeys returned error: %v", err)
	}

	subGroupManager := services.NewSubGroupManager(memStore)
	groupManager := services.NewGroupManager(db, memStore, settingsManager, subGroupManager)
	if err := groupManager.Initialize(); err != nil {
		t.Fatalf("Initialize group manager: %v", err)
	}
	defer groupManager.Stop(t.Context())

	channelFactory := channel.NewFactory(settingsManager, httpclient.NewHTTPClientManager())
	server, err := NewProxyServer(provider, groupManager, subGroupManager, settingsManager, channelFactory, nil, encryptionSvc)
	if err != nil {
		t.Fatalf("NewProxyServer returned error: %v", err)
	}

	router := gin.New()
	router.POST("/proxy/:group_name/*path", server.HandleProxy)

	body := []byte(`{"model":"gpt-4.1-nano","messages":[{"role":"user","content":"ping"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/proxy/proxy-success-reset/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	waitForProxyTestCondition(t, func() bool {
		var key models.APIKey
		if err := db.Where("group_id = ?", group.ID).First(&key).Error; err != nil {
			t.Fatalf("load key: %v", err)
		}
		return key.FailureCount == 0
	})
}

func openProxyTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Group{}, &models.APIKey{}, &models.GroupSubGroup{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func waitForProxyTestCondition(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !condition() {
		t.Fatal("condition was not met before timeout")
	}
}
