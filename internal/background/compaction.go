package background

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/data"
)

type CompactionDaemon interface {
	Run() error
}

type compactionDaemon struct {
	registryRepository data.RegistryRepository
	log                *log.Helper
}

func NewCompactionDaemon(registryRepository data.RegistryRepository, logger log.Logger) CompactionDaemon {
	return &compactionDaemon{
		registryRepository: registryRepository,
		log:                log.NewHelper(log.With(logger, "module", "background/CompactionDaemon")),
	}
}

func (d *compactionDaemon) Run() error {
	d.log.Infof("Running compaction")

	err := d.registryRepository.DeleteObsoleteDraftTags(context.Background())
	if err != nil {
		d.log.Errorf("DeleteObsoleteDraftTags error: %v", err)
		return err
	}

	d.log.Infof("Compaction finished")
	return nil
}
