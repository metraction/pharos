package main

// Util to list various image digest for a set of platforms
// Helps identify which digest we need for caching

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/metraction/pharos/internal/integrations/mq"
	"github.com/samber/lo"

	"github.com/rs/zerolog"
)

var err error
var logger zerolog.Logger
var cityList = []string{"London", "Paris", "Berlin", "Madrid", "Rome", "Vienna", "Budapest", "Prague", "Warsaw", "Bucharest", "Barcelona", "Munich", "Milan", "Hamburg", "Brussels", "Amsterdam", "Lisbon", "Stockholm", "Copenhagen", "Dublin", "Athens", "Helsinki", "Oslo", "Zurich", "Geneva", "Frankfurt", "Lyon", "Naples", "Turin", "Seville", "Valencia", "Stuttgart", "Düsseldorf", "Dortmund", "Essen", "Leipzig", "Bremen", "Dresden", "Hanover", "Nuremberg", "Duisburg", "Glasgow", "Birmingham", "Manchester", "Liverpool", "Edinburgh", "Sheffield", "Bristol", "Leeds", "Nottingham", "Leicester", "Newcastle", "Cardiff", "Belfast", "Sofia", "Belgrade", "Zagreb", "Ljubljana", "Bratislava", "Tallinn", "Riga", "Vilnius", "Luxembourg", "Monaco", "San Marino", "Andorra la Vella", "Vaduz", "Reykjavik", "Tirana", "Skopje", "Podgorica", "Pristina", "Sarajevo", "Split", "Dubrovnik", "Krakow", "Gdansk", "Poznan", "Wroclaw", "Katowice", "Lodz", "Szczecin", "Bydgoszcz", "Lublin", "Bialystok", "Plovdiv", "Varna", "Burgas", "Constanta", "Cluj-Napoca", "Timisoara", "Iasi", "Brasov", "Galati", "Ploiesti", "Oradea", "Chisinau", "St. Petersburg", "Moscow", "Kazan", "Nizhny Novgorod", "Samara", "Rostov-on-Don", "Ufa", "Volgograd", "Perm", "Krasnodar", "Saratov", "Voronezh", "Krasnoyarsk", "Sofia", "Thessaloniki", "Patras", "Heraklion", "Larissa", "Volos", "Chania", "Ioannina", "Kavala", "Trikala", "Piraeus", "Antwerp", "New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte", "San Francisco", "Indianapolis", "Seattle", "Denver", "Washington", "Boston", "El Paso", "Nashville", "Detroit", "Oklahoma City", "Portland", "Las Vegas", "Memphis", "Louisville", "Baltimore", "Milwaukee", "Albuquerque", "Tucson", "Fresno", "Sacramento", "Mesa", "Kansas City", "Atlanta", "Omaha", "Colorado Springs", "Raleigh", "Miami", "Long Beach", "Virginia Beach", "Oakland", "Minneapolis", "Tulsa", "Arlington", "Tampa", "New Orleans", "Wichita", "Cleveland", "Bakersfield", "Aurora", "Anaheim", "Honolulu", "Santa Ana", "Riverside", "Corpus Christi", "Lexington", "Henderson", "Stockton", "Saint Paul", "Cincinnati", "St. Louis", "Pittsburgh", "Greensboro", "Lincoln", "Anchorage", "Plano", "Orlando"}

type CityType struct {
	Id      int
	Name    string
	Created time.Time
	Trigger int
}

func init() {
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	logger = zerolog.New(consoleWriter).With().Timestamp().Logger()

	err = godotenv.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("")
	}

}

// handler to process cities
func cityHandlerFunc(x mq.TaskMessage[CityType]) error {

	k := x.Data.Id%10 + 1
	delay := time.Duration(100*k) * time.Millisecond

	logger.Info().
		Str("m.id", x.Id).
		Any("m.retry", x.RetryCount).
		Any("m.idle", x.IdleTime.Seconds()).
		Any("city.id", x.Data.Id).
		Any("city.tr", x.Data.Trigger).
		Any("city.name", x.Data.Name).
		Any("t.sleep", delay.Seconds()).
		Msg("received")

	time.Sleep(delay)
	if x.Data.Id%10 == 0 {
		return fmt.Errorf("scan failed (systematic)")
	}

	if x.Data.Trigger > 0 {
		return nil
	}
	if x.RetryCount > 2 {
		return nil
	}
	return fmt.Errorf("scan failed (sporadic)")
}

func main() {

	action := flag.String("action", "", "action [tx,rx]")
	redisEndpoint := flag.String("endpoint", "redis://:pwd@localhost:6379/0", "Redis endpoint")
	consumer := flag.String("consumer", "", "Consumer name")
	samples := flag.Int("samples", 5, "number of samples")

	flag.Parse()

	ctx := context.Background()

	for k := 1; k <= 10; k++ {
		cityList = append(cityList, cityList...)
	}

	streamName := "mx"
	groupName := "gx"
	consumerName := *consumer

	logger.Info().Msg("-----< Message Queue Testing >-----")
	logger.Info().
		Str("groupName", groupName).
		Str("streamName", streamName).
		Str("consumerName", consumerName).
		Str("action", *action).
		Any("sanokes", *samples).
		Str("redisEndpoint", *redisEndpoint).
		Msg("")

	// stream dimension
	maxStreamLen := int64(100000) // max # messages in stream

	// message limitations
	maxMsgRetry := int64(4)       // delete tasks after this may retries
	maxMsgTTL := 45 * time.Second // tasks idle longer than this are deleted

	// cleanup process
	cleanupInterval := 15 * time.Second  // intervall to execute stale/expired/cleanup check
	reclaimOlderThan := 30 * time.Second // get and reclaim messages with idleTime > this
	deleteOlderThan := 60 * time.Second  // get and delete messages with idleTime > this
	var reclaimStaleBlock int64 = 20     // reclaim buffer (number of messages) to reclaim per call
	var removeStaleBlock int64 = 20      // delete buffer (number of messages) to delete per call

	var err error
	var tmq *mq.RedisTaskQueue[CityType]

	// setup redis task queue
	if tmq, err = mq.NewRedisTaskQueue[CityType](ctx, *redisEndpoint, streamName, maxStreamLen, maxMsgRetry, maxMsgTTL); err != nil {
		logger.Fatal().Err(err).Msg("NewRedisGtrsClientStefan()")
	}
	if err = tmq.Connect(ctx); err != nil {
		logger.Fatal().Err(err).Msg("tmq.Connec()")
	}
	defer tmq.Close()

	// ensure the stream is created with defined settings if it not exists
	if err = tmq.CreateGroup(ctx, groupName, "$"); err != nil {
		logger.Fatal().Err(err).Msg("tmq.CreateGroup()")
	}

	// check stale and reclaim every 10 seconds
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	// TODO: Ensure short ticker time does not start task before previous task finished
	// TODO: Enforce timeouts
	go func() {
		logger.Info().Msg("Start RemoveStale Checker")
		for range ticker.C {
			logger.Info().Str("time", time.Now().Format("15:04:05")).Msg("Ticker")
			if _, err = tmq.ReclaimStale(ctx, groupName, consumerName, reclaimStaleBlock, reclaimOlderThan, cityHandlerFunc); err != nil {
				logger.Error().Err(err).Msg("ReclaimStale")
			}
			if _, err = tmq.RemoveStale(ctx, groupName, removeStaleBlock, deleteOlderThan); err != nil {
				logger.Error().Err(err).Msg("RemoveStale")
			}
		}
	}()

	// action TX
	if *action == "tx" || *action == "trx" {
		logger.Info().Msg("-----< Action:send [tx] >-----")

		for k, name := range lo.Slice(cityList, 0, *samples) {
			city := CityType{Id: k, Name: name, Created: time.Now(), Trigger: k % 3}

			id, err := tmq.AddMessage(ctx, 1, city)
			if err != nil {
				logger.Fatal().Err(err).Any("k", k).Str("name", name).Msg("")
			}
			logger.Info().Any("k", k).Str("name", city.Name).Any("trigger", city.Trigger).Str("mid", id).Msg("")
		}
		logger.Info().Msg("done tx")
	}
	// action RX
	if *action == "rx" || *action == "trx" {
		logger.Info().Msg("-----< Action:read [rx] >-----")

		err := tmq.GroupSubscribe(ctx, ">", groupName, consumerName, 0*time.Second, cityHandlerFunc)
		if err != nil {
			logger.Fatal().Err(err).Msg("GroupSubscribe")
		}
		logger.Info().Msg("done rx")
	}

	logger.Info().
		Str("time", time.Now().String()).
		Msg("done")
}
