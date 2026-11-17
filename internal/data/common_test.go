package data

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"sync"
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
	pool               *pgxpool.Pool
	registryRepository RegistryRepository
	metadataRepository MetadataRepository
	userRepository     UserRepository
	aclRepository      ACLRepository
	driftRepository    DriftRepository
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

	// Exercise concurrent multi-replica startup migrations serialized by PostgreSQL session advisory lock.
	var wg sync.WaitGroup
	migErrs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db, openErr := sql.Open("pgx", dsn)
			if openErr != nil {
				migErrs <- openErr
				return
			}
			defer func() { _ = db.Close() }()
			if migErr := migrations.MigrateWithError(ctx, db); migErr != nil {
				migErrs <- migErr
			}
		}()
	}
	wg.Wait()
	close(migErrs)
	for migErr := range migErrs {
		if migErr != nil {
			panic(migErr)
		}
	}

	s.pool = pool
	s.registryRepository = NewRegistryRepository(pool, log.DefaultLogger)
	s.metadataRepository = NewMetadataRepository(pool, log.DefaultLogger)
	s.userRepository = NewUserRepository(pool, log.DefaultLogger)
	s.aclRepository = NewACLRepository(pool, log.DefaultLogger)
	s.driftRepository = NewDriftRepository(pool, log.DefaultLogger)
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
	if testing.Short() {
		os.Exit(0)
	}
	suite.SetupSuite()
	code := m.Run()
	suite.TearDownSuite()
	os.Exit(code)
}
