package repository

import (
	"errors"
	"strings"
	"testing"

	"github.com/SosnovichIvan/arhdesign/apps/api/internal/account"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/globalchat"
	"github.com/SosnovichIvan/arhdesign/apps/api/internal/project"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapProjectWriteErrorClassifiesIntegrityFailures(t *testing.T) {
	for _, code := range []string{"23505", "23514"} {
		err := mapProjectWriteError("write", &pgconn.PgError{Code: code})
		if !errors.Is(err, project.ErrConflict) {
			t.Fatalf("code=%s error=%v", code, err)
		}
	}
	if err := mapProjectWriteError("write", &pgconn.PgError{Code: "23503"}); !errors.Is(err, project.ErrAccountUnavailable) {
		t.Fatalf("foreign key error=%v", err)
	}
	original := errors.New("network")
	if err := mapProjectWriteError("write", original); !errors.Is(err, original) {
		t.Fatalf("generic error=%v", err)
	}
}

func TestRepositoryHelperErrorMappingsAndStringNormalization(t *testing.T) {
	unique := &pgconn.PgError{Code: "23505"}
	if err := mapAccountBootstrapWriteError("bootstrap", unique); !errors.Is(err, account.ErrIdentifierUnavailable) {
		t.Fatalf("account bootstrap unique error=%v", err)
	}
	if err := mapGlobalChatWriteError("chat", unique); !errors.Is(err, globalchat.ErrConflict) {
		t.Fatalf("global chat unique error=%v", err)
	}
	original := errors.New("database unavailable")
	if err := mapAccountBootstrapWriteError("bootstrap", original); !errors.Is(err, original) || !strings.Contains(err.Error(), "bootstrap") {
		t.Fatalf("account bootstrap generic error=%v", err)
	}
	if err := mapGlobalChatWriteError("chat", original); !errors.Is(err, original) || !strings.Contains(err.Error(), "chat") {
		t.Fatalf("global chat generic error=%v", err)
	}
	if valueOrEmpty(nil) != "" {
		t.Fatal("nil optional string must normalize to empty")
	}
	value := "Фамилия"
	if valueOrEmpty(&value) != value {
		t.Fatal("non-empty optional string changed")
	}
	if escaped := escapeGlobalChatLike(`a%b_c\\d`); escaped != `a\%b\_c\\\\d` {
		t.Fatalf("escaped LIKE value=%q", escaped)
	}
}

func TestMapContextEntityReadError(t *testing.T) {
	if err := mapContextEntityReadError("material", pgx.ErrNoRows); !errors.Is(err, project.ErrNotFound) {
		t.Fatalf("no rows error=%v", err)
	}
	original := errors.New("database unavailable")
	if err := mapContextEntityReadError("expense", original); !errors.Is(err, original) {
		t.Fatalf("wrapped error=%v", err)
	}
}
