package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/sotah-inc/steamwheedle-cartel-server/app/commands"
	devCommand "github.com/sotah-inc/steamwheedle-cartel/pkg/command/dev"
	prodCommand "github.com/sotah-inc/steamwheedle-cartel/pkg/command/prod"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/logging"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/logging/stackdriver"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/sotah"
	devState "github.com/sotah-inc/steamwheedle-cartel/pkg/state/dev"
	prodState "github.com/sotah-inc/steamwheedle-cartel/pkg/state/prod"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/store"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/store/regions"
	"github.com/twinj/uuid"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

type commandMap map[string]func() error

// ID represents this run's unique id
var ID uuid.UUID

func main() {
	// assigning global ID
	ID = uuid.NewV4()

	// parsing the command flags
	var (
		app            = kingpin.New("sotah-server", "A command-line Blizzard AH client.")
		natsHost       = app.Flag("nats-host", "NATS hostname").Default("localhost").Envar("NATS_HOST").Short('h').String()
		natsPort       = app.Flag("nats-port", "NATS port").Default("4222").Envar("NATS_PORT").Short('p').Int()
		clientID       = app.Flag("client-id", "Blizzard API Client ID").Envar("CLIENT_ID").String()
		clientSecret   = app.Flag("client-secret", "Blizzard API Client Secret").Envar("CLIENT_SECRET").String()
		verbosity      = app.Flag("verbosity", "Log verbosity").Default("info").Short('v').String()
		cacheDir       = app.Flag("cache-dir", "Directory to cache data files to").Required().String()
		projectID      = app.Flag("project-id", "GCloud Storage Project ID").Default("").Envar("PROJECT_ID").String()
		isLocal        = app.Flag("is-local", "Flag to use local config filepath or not").Bool()
		configFilepath = app.Flag("config-filepath", "Optional config filepath").Short('c').String()

		apiCommand                = app.Command(string(commands.API), "For running sotah-server.")
		liveAuctionsCommand       = app.Command(string(commands.LiveAuctions), "For in-memory storage of current auctions.")
		pricelistHistoriesCommand = app.Command(string(commands.PricelistHistories), "For on-disk storage of pricelist histories.")

		prodApiCommand                = app.Command(string(commands.ProdApi), "For running sotah-server in prod-mode.")
		prodMetricsCommand            = app.Command(string(commands.ProdMetrics), "For forwarding metrics to a nats channel.")
		prodLiveAuctionsCommand       = app.Command(string(commands.ProdLiveAuctions), "For managing live-auctions in gcp ce vm.")
		prodPricelistHistoriesCommand = app.Command(string(commands.ProdPricelistHistories), "For managing pricelist-histories in gcp ce vm.")
		prodItemsCommand              = app.Command(string(commands.ProdItems), "For managing items in gcp ce vm.")
		prodGateway                   = app.Command(string(commands.ProdGateway), "For invoking the act gateway.")
		prodPubsubTopicsMonitor       = app.Command(string(commands.ProdPubsubTopicsMonitor), "For invoking the pubsub-topics-monitor gateway.")
	)
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	// establishing log verbosity
	logVerbosity, err := logrus.ParseLevel(*verbosity)
	if err != nil {
		logging.WithField("error", err.Error()).Fatal("Could not parse log level")

		return
	}
	logging.SetLevel(logVerbosity)

	// gathering the config file from a store
	c, err := func() (sotah.Config, error) {
		if *isLocal {
			return sotah.NewConfigFromFilepath(*configFilepath)
		}

		storeClient, err := store.NewClient(*projectID)
		if err != nil {
			return sotah.Config{}, err
		}

		bootBase := store.NewBootBase(storeClient, regions.USCentral1)
		bootBucket, err := bootBase.GetFirmBucket()
		configObj, err := bootBase.GetFirmObject("config.json", bootBucket)
		if err != nil {
			return sotah.Config{}, err
		}

		reader, err := configObj.NewReader(storeClient.Context)
		if err != nil {
			return sotah.Config{}, err
		}

		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return sotah.Config{}, err
		}

		var out sotah.Config
		if err := json.Unmarshal(data, &out); err != nil {
			return sotah.Config{}, err
		}

		return out, nil
	}()
	if err != nil {
		logging.WithField("error", err.Error()).Fatal("Could not gather config from store")

		return
	}

	// optionally adding stackdriver hook
	if c.UseGCloud {
		logging.WithField("project-id", *projectID).Info("Creating stackdriver hook")
		stackdriverHook, err := stackdriver.NewHook(*projectID, cmd)
		if err != nil {
			logging.WithFields(logrus.Fields{
				"error":     err.Error(),
				"projectID": projectID,
			}).Fatal("Could not create new stackdriver logrus hook")

			return
		}

		logging.AddHook(stackdriverHook)
	}
	logging.Info("Starting")

	logging.WithField("command", cmd).Info("Running command")

	// declaring a command map
	cMap := commandMap{
		apiCommand.FullCommand(): func() error {
			return devCommand.Api(devState.APIStateConfig{
				SotahConfig:          c,
				DiskStoreCacheDir:    *cacheDir,
				ItemsDatabaseDir:     fmt.Sprintf("%s/databases", *cacheDir),
				BlizzardClientSecret: *clientSecret,
				BlizzardClientId:     *clientID,
				MessengerPort:        *natsPort,
				MessengerHost:        *natsHost,
				GCloudProjectID:      *projectID,
			})
		},
		liveAuctionsCommand.FullCommand(): func() error {
			return devCommand.LiveAuctions(devState.LiveAuctionsStateConfig{
				MessengerHost:           *natsHost,
				MessengerPort:           *natsPort,
				DiskStoreCacheDir:       *cacheDir,
				LiveAuctionsDatabaseDir: fmt.Sprintf("%s/databases", *cacheDir),
			})
		},
		pricelistHistoriesCommand.FullCommand(): func() error {
			return devCommand.PricelistHistories(devState.PricelistHistoriesStateConfig{
				DiskStoreCacheDir:             *cacheDir,
				MessengerPort:                 *natsPort,
				MessengerHost:                 *natsHost,
				PricelistHistoriesDatabaseDir: fmt.Sprintf("%s/databases/pricelist-histories", *cacheDir),
			})
		},
		prodApiCommand.FullCommand(): func() error {
			return prodCommand.ProdApi(prodState.ProdApiStateConfig{
				SotahConfig:     c,
				MessengerPort:   *natsPort,
				MessengerHost:   *natsHost,
				GCloudProjectID: *projectID,
			})
		},
		prodMetricsCommand.FullCommand(): func() error {
			return prodCommand.ProdMetrics(prodState.ProdMetricsStateConfig{
				MessengerPort:   *natsPort,
				MessengerHost:   *natsHost,
				GCloudProjectID: *projectID,
			})
		},
		prodLiveAuctionsCommand.FullCommand(): func() error {
			return prodCommand.ProdLiveAuctions(prodState.ProdLiveAuctionsStateConfig{
				MessengerPort:           *natsPort,
				MessengerHost:           *natsHost,
				GCloudProjectID:         *projectID,
				LiveAuctionsDatabaseDir: fmt.Sprintf("%s/databases", *cacheDir),
			})
		},
		prodPricelistHistoriesCommand.FullCommand(): func() error {
			return prodCommand.ProdPricelistHistories(prodState.ProdPricelistHistoriesStateConfig{
				MessengerPort:                 *natsPort,
				MessengerHost:                 *natsHost,
				GCloudProjectID:               *projectID,
				PricelistHistoriesDatabaseDir: fmt.Sprintf("%s/databases", *cacheDir),
			})
		},
		prodItemsCommand.FullCommand(): func() error {
			return prodCommand.Items(prodState.ItemsStateConfig{
				MessengerPort:    *natsPort,
				MessengerHost:    *natsHost,
				GCloudProjectID:  *projectID,
				ItemsDatabaseDir: fmt.Sprintf("%s/databases", *cacheDir),
			})
		},
		prodGateway.FullCommand(): func() error {
			return prodCommand.Gateway(prodState.GatewayStateConfig{
				ProjectId: *projectID,
			})
		},
		prodPubsubTopicsMonitor.FullCommand(): func() error {
			return prodCommand.PubsubTopicsMonitor(prodState.PubsubTopicsMonitorStateConfig{
				ProjectID:               *projectID,
				MessengerPort:           *natsPort,
				MessengerHost:           *natsHost,
				PubsubTopicsDatabaseDir: fmt.Sprintf("%s/databases", *cacheDir),
			})
		},
	}

	// resolving the command func
	cmdFunc, ok := cMap[cmd]
	if !ok {
		logging.WithField("command", cmd).Fatal("Invalid command")

		return
	}

	// calling the command func
	if err := cmdFunc(); err != nil {
		logging.WithFields(logrus.Fields{
			"error":   err.Error(),
			"command": cmd,
		}).Fatal("Failed to execute command")
	}
}
