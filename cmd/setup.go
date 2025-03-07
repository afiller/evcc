package cmd

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/evcc-io/evcc/api"
	"github.com/evcc-io/evcc/cmd/shutdown"
	"github.com/evcc-io/evcc/core"
	"github.com/evcc-io/evcc/core/loadpoint"
	"github.com/evcc-io/evcc/hems"
	"github.com/evcc-io/evcc/provider/javascript"
	"github.com/evcc-io/evcc/provider/mqtt"
	"github.com/evcc-io/evcc/push"
	"github.com/evcc-io/evcc/server"
	"github.com/evcc-io/evcc/tariff"
	"github.com/evcc-io/evcc/util"
	"github.com/evcc-io/evcc/util/pipe"
	"github.com/evcc-io/evcc/util/sponsor"
	"github.com/spf13/viper"
	"golang.org/x/text/currency"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var cp = new(ConfigProvider)

func loadConfigFile(cfgFile string, conf *config) (err error) {
	if cfgFile != "" {
		log.INFO.Println("using config file", cfgFile)
		if err := viper.UnmarshalExact(&conf); err != nil {
			log.FATAL.Fatalf("failed parsing config file %s: %v", cfgFile, err)
		}
	} else {
		err = errors.New("missing evcc config")
	}

	return err
}

func configureEnvironment(conf config) (err error) {
	// setup sponsorship
	if conf.SponsorToken != "" {
		err = sponsor.ConfigureSponsorship(conf.SponsorToken)
	}

	// setup mqtt client listener
	if err == nil && conf.Mqtt.Broker != "" {
		err = configureMQTT(conf.Mqtt)
	}

	// setup javascript VMs
	if err == nil {
		err = configureJavascript(conf.Javascript)
	}

	// setup EEBus server
	if err == nil && conf.EEBus != nil {
		err = configureEEBus(conf.EEBus)
	}

	return
}

// setup influx database
func configureDatabase(conf server.InfluxConfig, loadPoints []loadpoint.API, in <-chan util.Param) {
	influx := server.NewInfluxClient(
		conf.URL,
		conf.Token,
		conf.Org,
		conf.User,
		conf.Password,
		conf.Database,
	)

	// eliminate duplicate values
	dedupe := pipe.NewDeduplicator(30*time.Minute, "vehicleCapacity", "vehicleSoC", "vehicleRange", "vehicleOdometer", "chargedEnergy", "chargeRemainingEnergy")
	in = dedupe.Pipe(in)

	// reduce number of values written to influx
	// TODO this breaks writing vehicleRange as its re-writting in short interval
	// limiter := pipe.NewLimiter(5 * time.Second)
	// in = limiter.Pipe(in)

	go influx.Run(loadPoints, in)
}

// setup mqtt
func configureMQTT(conf mqttConfig) error {
	log := util.NewLogger("mqtt")

	var err error
	mqtt.Instance, err = mqtt.RegisteredClient(log, conf.Broker, conf.User, conf.Password, conf.ClientID, 1, conf.Insecure, func(options *paho.ClientOptions) {
		topic := fmt.Sprintf("%s/status", conf.RootTopic())
		options.SetWill(topic, "offline", 1, true)
	})
	if err != nil {
		return fmt.Errorf("failed configuring mqtt: %w", err)
	}

	return nil
}

// setup javascript
func configureJavascript(conf map[string]interface{}) error {
	if err := javascript.Configure(conf); err != nil {
		return fmt.Errorf("failed configuring javascript: %w", err)
	}
	return nil
}

// setup HEMS
func configureHEMS(conf typedConfig, site *core.Site, httpd *server.HTTPd) hems.HEMS {
	hems, err := hems.NewFromConfig(conf.Type, conf.Other, site, httpd)
	if err != nil {
		log.FATAL.Fatalf("failed configuring hems: %v", err)
	}
	return hems
}

// setup EEBus
func configureEEBus(conf map[string]interface{}) error {
	var err error
	if server.EEBusInstance, err = server.NewEEBus(conf); err == nil {
		go server.EEBusInstance.Run()
		shutdown.Register(server.EEBusInstance.Shutdown)
	}

	return nil
}

// setup messaging
func configureMessengers(conf messagingConfig, cache *util.Cache) chan push.Event {
	notificationChan := make(chan push.Event, 1)
	notificationHub, err := push.NewHub(conf.Events, cache)
	if err != nil {
		log.FATAL.Fatalf("failed configuring push services: %v", err)
	}

	for _, service := range conf.Services {
		impl, err := push.NewMessengerFromConfig(service.Type, service.Other)
		if err != nil {
			log.FATAL.Fatal(err)
			log.FATAL.Fatalf("failed configuring messenger %s: %v", service.Type, err)
		}
		notificationHub.Add(impl)
	}

	go notificationHub.Run(notificationChan)

	return notificationChan
}

func configureTariffs(conf tariffConfig) (tariff.Tariffs, error) {
	var grid, feedin api.Tariff
	var currencyCode currency.Unit = currency.EUR
	var err error

	if conf.Currency != "" {
		currencyCode = currency.MustParseISO(conf.Currency)
	}

	if conf.Grid.Type != "" {
		grid, err = tariff.NewFromConfig(conf.Grid.Type, conf.Grid.Other)
	}

	if err == nil && conf.FeedIn.Type != "" {
		feedin, err = tariff.NewFromConfig(conf.FeedIn.Type, conf.FeedIn.Other)
	}

	if err != nil {
		err = fmt.Errorf("failed configuring tariff: %w", err)
	}

	tariffs := tariff.NewTariffs(currencyCode, grid, feedin)

	return *tariffs, err
}

func configureSiteAndLoadpoints(conf config) (site *core.Site, err error) {
	if err = cp.configure(conf); err == nil {
		var loadPoints []*core.LoadPoint
		loadPoints, err = configureLoadPoints(conf, cp)

		var tariffs tariff.Tariffs
		if err == nil {
			tariffs, err = configureTariffs(conf.Tariffs)
		}

		if err == nil {
			site, err = configureSite(conf.Site, cp, loadPoints, tariffs)
		}
	}

	return site, err
}

func configureSite(conf map[string]interface{}, cp *ConfigProvider, loadPoints []*core.LoadPoint, tariffs tariff.Tariffs) (*core.Site, error) {
	site, err := core.NewSiteFromConfig(log, cp, conf, loadPoints, tariffs)
	if err != nil {
		return nil, fmt.Errorf("failed configuring site: %w", err)
	}

	return site, nil
}

func configureLoadPoints(conf config, cp *ConfigProvider) (loadPoints []*core.LoadPoint, err error) {
	lpInterfaces, ok := viper.AllSettings()["loadpoints"].([]interface{})
	if !ok || len(lpInterfaces) == 0 {
		return nil, errors.New("missing loadpoints")
	}

	for id, lpcI := range lpInterfaces {
		var lpc map[string]interface{}
		if err := util.DecodeOther(lpcI, &lpc); err != nil {
			return nil, fmt.Errorf("failed decoding loadpoint configuration: %w", err)
		}

		log := util.NewLogger("lp-" + strconv.Itoa(id+1))
		lp, err := core.NewLoadPointFromConfig(log, cp, lpc)
		if err != nil {
			return nil, fmt.Errorf("failed configuring loadpoint: %w", err)
		}

		loadPoints = append(loadPoints, lp)
	}

	return loadPoints, nil
}
