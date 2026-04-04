//go:build integration

package http_test

import (
	"context"
	"os"
	"testing"

	"github.com/zambone/pfm-go/internal/testutil/dbtest"
)

var sharedDB *dbtest.SharedDB

func TestMain(m *testing.M) {
	ctx := context.Background()
	var cleanup func()
	sharedDB, cleanup = dbtest.Setup(ctx)
	defer cleanup()
	sharedDB.PrepareTemplate(ctx)
	os.Exit(m.Run())
}
