package data

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pbufio/pbuf-registry/migrations"
	"github.com/pbufio/pbuf-registry/test_utils"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var suite TestSuite

type TestSuite struct {
	psqlContainer      *test_utils.PostgreSQLContainer
	registryRepository RegistryRepository
}

func (s *TestSuite) SetupSuite() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ctxCancel()

	psqlContainer, err := test_utils.NewPostgreSQLContainer(ctx)
	if err != nil {
		panic(err)
	}

	s.psqlContainer = psqlContainer

	// waiting for 5 secs to be sure that container
	// can accept connections
	time.Sleep(5 * time.Second)

	dsn := s.psqlContainer.GetDSN()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	migrations.Migrate(db)

	s.registryRepository = NewRegistryRepository(pool, log.DefaultLogger)
}

func (s *TestSuite) TearDownSuite() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	err := s.psqlContainer.Terminate(ctx)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	suite.SetupSuite()
	code := m.Run()
	suite.TearDownSuite()
	os.Exit(code)
}
