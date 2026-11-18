package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"gojo/infrastructure/mysql"
	"gojo/internal/submission/model"
	"gojo/internal/submission/repository"
	"gojo/internal/submission/service"
)

func TestSubmissionDetailScopesParameterizedQueryToCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(mysqlDriver.New(mysqlDriver.Config{
		DSN: "test@tcp(127.0.0.1:3306)/test", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	previous := mysql.DB
	mysql.DB = db
	t.Cleanup(func() {
		mysql.DB = previous
		_ = sqlDB.Close()
	})

	var queries int
	err = db.Callback().Query().After("gorm:query").Register("test:submission_lookup", func(tx *gorm.DB) {
		queries++
		args := tx.Statement.Vars
		if !strings.Contains(tx.Statement.SQL.String(), "id = ? AND user_id = ?") || len(args) < 2 {
			t.Fatalf("expected a parameterized lookup by submission and owner, got %s", tx.Statement.SQL.String())
		}
		if strings.Contains(tx.Statement.SQL.String(), "actual_output") {
			t.Fatal("submission detail selected stored judge output")
		}
		if args[0] == uint(17) && args[1] == uint(7) {
			sub := tx.Statement.Dest.(*model.Submission)
			sub.ID, sub.UserID, sub.Status = 17, 7, "WA"
			sub.ActualOutput = "legacy hidden output marker"
			return
		}
		_ = tx.AddError(gorm.ErrRecordNotFound)
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := NewSubmissionHandler(service.NewSubmissionService(repository.NewSubmissionRepository(), nil))
	router := gin.New()
	router.GET("/submissions/:id", func(c *gin.Context) {
		userID := uint(7)
		if c.Query("other") == "true" {
			userID = 8
		}
		c.Set("userID", userID)
		handler.GetSubmissionResult(c)
	})
	for _, test := range []struct {
		name, path string
		status     int
	}{
		{"own submission", "/submissions/17", http.StatusOK},
		{"another user's submission", "/submissions/17?other=true", http.StatusNotFound},
		{"missing submission", "/submissions/18", http.StatusNotFound},
		{"invalid id", "/submissions/invalid", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := queries
			result := httptest.NewRecorder()
			router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, test.path, nil))
			if result.Code != test.status {
				t.Fatalf("status=%d, want=%d", result.Code, test.status)
			}
			if test.status == http.StatusBadRequest && queries != before {
				t.Fatal("invalid ID reached the database query")
			}
			if test.status == http.StatusOK && (strings.Contains(result.Body.String(), "legacy hidden output marker") ||
				!strings.Contains(result.Body.String(), "输出与预期不符")) {
				t.Fatalf("expected a fixed verdict message, got %s", result.Body.String())
			}
		})
	}
}
