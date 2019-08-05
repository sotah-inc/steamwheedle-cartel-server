package prod

import (
	"github.com/sotah-inc/steamwheedle-cartel/pkg/act"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/bus"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/bus/codes"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/logging"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/sotah"
	"github.com/sotah-inc/steamwheedle-cartel/pkg/state/subjects"
)

func (sta GatewayState) RunComputeAllPricelistHistories(tuples sotah.RegionRealmTimestampTuples) error {
	// generating an act client
	logging.WithField("endpoint-url", sta.actEndpoints.Gateway).Info("Producing act client")
	actClient, err := act.NewClient(sta.actEndpoints.Gateway)
	if err != nil {
		return err
	}

	// calling compute-all-pricelist-histories on gateway service
	logging.Info("Calling compute-all-pricelist-histories on gateway service")
	if err := actClient.ComputeAllPricelistHistories(tuples); err != nil {
		return err
	}

	logging.Info("Done calling compute-all-pricelist-histories")

	return nil
}

func (sta GatewayState) ListenForCallComputeAllPricelistHistories(
	onReady chan interface{},
	stop chan interface{},
	onStopped chan interface{},
) {
	in := make(chan sotah.RegionRealmTimestampTuples)
	go func() {
		for tuples := range in {
			if err := sta.RunComputeAllPricelistHistories(tuples); err != nil {
				logging.WithField("error", err.Error()).Error("Failed to call RunComputeAllPricelistHistories()")

				continue
			}
		}
	}()

	// establishing subscriber config
	config := bus.SubscribeConfig{
		Stop: stop,
		Callback: func(busMsg bus.Message) {
			logging.WithField("bus-msg", busMsg).Info("Received bus-message")

			// parsing the message body
			tuples, err := sotah.NewRegionRealmTimestampTuples(busMsg.Data)
			if err != nil {
				logging.WithField("error", err.Error()).Error("Failed to parse bus message body")

				if err := sta.IO.BusClient.ReplyToWithError(busMsg, err, codes.GenericError); err != nil {
					logging.WithField("error", err.Error()).Error("Failed to reply to message")

					return
				}

				return
			}

			// acking the message
			if _, err := sta.IO.BusClient.ReplyTo(busMsg, bus.NewMessage()); err != nil {
				logging.WithField("error", err.Error()).Error("Failed to reply to message")

				return
			}

			in <- tuples
		},
		OnReady:   onReady,
		OnStopped: onStopped,
	}

	// starting up worker for the subscription
	go func() {
		if err := sta.IO.BusClient.SubscribeToTopic(string(subjects.CallComputeAllPricelistHistories), config); err != nil {
			logging.WithField("error", err.Error()).Fatal("Failed to subscribe to topic")
		}
	}()
}
