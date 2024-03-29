package dev

import (
	"fmt"

	"github.com/sotah-inc/steamwheedle-cartel/pkg/messenger"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/metric"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/sotah"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/state"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/util"
	"github.com/twinj/uuid"
)

type PricelistHistoriesStateConfig struct {
	MessengerHost string
	MessengerPort int

	DiskStoreCacheDir string

	PricelistHistoriesDatabaseDir string
}

func NewPricelistHistoriesState(config PricelistHistoriesStateConfig) (PricelistHistoriesState, error) {
	phState := PricelistHistoriesState{
		State: state.NewState(uuid.NewV4(), false),
	}
	phState.Statuses = sotah.Statuses{}

	// connecting to the messenger host
	mess, err := messenger.NewMessenger(config.MessengerHost, config.MessengerPort)
	if err != nil {
		return PricelistHistoriesState{}, err
	}
	phState.IO.Messenger = mess

	// initializing a reporter
	phState.IO.Reporter = metric.NewReporter(mess)

	// gathering regions
	regions, err := phState.NewRegions()
	if err != nil {
		return PricelistHistoriesState{}, err
	}
	phState.Regions = regions

	// gathering statuses
	for _, reg := range phState.Regions {
		status, err := phState.IO.Messenger.NewStatus(reg)
		if err != nil {
			return PricelistHistoriesState{}, err
		}

		phState.Statuses[reg.Name] = status
	}

	// ensuring database paths exist
	databasePaths := []string{}
	for regionName, status := range phState.Statuses {
		for _, realm := range status.Realms {
			databasePaths = append(databasePaths, fmt.Sprintf(
				"%s/pricelist-histories/%s/%s",
				config.PricelistHistoriesDatabaseDir,
				regionName,
				realm.Slug,
			))
		}
	}
	if err := util.EnsureDirsExist(databasePaths); err != nil {
		return PricelistHistoriesState{}, err
	}

	return phState, nil
}

type PricelistHistoriesState struct {
	state.State

	Regions  sotah.RegionList
	Statuses sotah.Statuses
}
