package main

// Util to list various image digest for a set of platforms
// Helps identify which digest we need for caching

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/dranikpg/gtrs"
	"github.com/joho/godotenv"
	"github.com/metraction/pharos/scripts/sender/mq"

	"github.com/rs/zerolog"
)

var err error
var logger zerolog.Logger
var cityList = []string{"London", "Paris", "Berlin", "Madrid", "Rome", "Vienna", "Budapest", "Prague", "Warsaw", "Bucharest", "Barcelona", "Munich", "Milan", "Hamburg", "Brussels", "Amsterdam", "Lisbon", "Stockholm", "Copenhagen", "Dublin", "Athens", "Helsinki", "Oslo", "Zurich", "Geneva", "Frankfurt", "Lyon", "Naples", "Turin", "Seville", "Valencia", "Stuttgart", "Düsseldorf", "Dortmund", "Essen", "Leipzig", "Bremen", "Dresden", "Hanover", "Nuremberg", "Duisburg", "Glasgow", "Birmingham", "Manchester", "Liverpool", "Edinburgh", "Sheffield", "Bristol", "Leeds", "Nottingham", "Leicester", "Newcastle", "Cardiff", "Belfast", "Sofia", "Belgrade", "Zagreb", "Ljubljana", "Bratislava", "Tallinn", "Riga", "Vilnius", "Luxembourg", "Monaco", "San Marino", "Andorra la Vella", "Vaduz", "Reykjavik", "Tirana", "Skopje", "Podgorica", "Pristina", "Sarajevo", "Split", "Dubrovnik", "Krakow", "Gdansk", "Poznan", "Wroclaw", "Katowice", "Lodz", "Szczecin", "Bydgoszcz", "Lublin", "Bialystok", "Plovdiv", "Varna", "Burgas", "Constanta", "Cluj-Napoca", "Timisoara", "Iasi", "Brasov", "Galati", "Ploiesti", "Oradea", "Chisinau", "St. Petersburg", "Moscow", "Kazan", "Nizhny Novgorod", "Samara", "Rostov-on-Don", "Ufa", "Volgograd", "Perm", "Krasnodar", "Saratov", "Voronezh", "Krasnoyarsk", "Sofia", "Thessaloniki", "Patras", "Heraklion", "Larissa", "Volos", "Chania", "Ioannina", "Kavala", "Trikala", "Piraeus", "Antwerp", "New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte", "San Francisco", "Indianapolis", "Seattle", "Denver", "Washington", "Boston", "El Paso", "Nashville", "Detroit", "Oklahoma City", "Portland", "Las Vegas", "Memphis", "Louisville", "Baltimore", "Milwaukee", "Albuquerque", "Tucson", "Fresno", "Sacramento", "Mesa", "Kansas City", "Atlanta", "Omaha", "Colorado Springs", "Raleigh", "Miami", "Long Beach", "Virginia Beach", "Oakland", "Minneapolis", "Tulsa", "Arlington", "Tampa", "New Orleans", "Wichita", "Cleveland", "Bakersfield", "Aurora", "Anaheim", "Honolulu", "Santa Ana", "Riverside", "Corpus Christi", "Lexington", "Henderson", "Stockton", "Saint Paul", "Cincinnati", "St. Louis", "Pittsburgh", "Greensboro", "Lincoln", "Anchorage", "Plano", "Orlando"}

type CityType struct {
	Id      int
	Created time.Time
	Name    string
	Trigger int
}
type DummyType int

func init() {
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	logger = zerolog.New(consoleWriter).With().Timestamp().Logger()

	err = godotenv.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("")
	}

}

func main() {

	sleep := flag.Duration("sleep", 500*time.Millisecond, "sleep duration")
	action := flag.String("action", "", "action [tx,rx]")
	samples := flag.Int("samples", 3, "number of samples to send")
	errors := flag.Int("errors", 0, "trigger error in handler for every N tasks (0=no errors)")
	consumerName := flag.String("consumer", "", "consumer name for reading")
	redisEndpoint := flag.String("endpoint", "redis://:pwd@localhost:6379/0", "Redis endpoint")

	flag.Parse()

	ctx := context.Background()

	streamName := "mq"
	groupName := "scan"

	logger.Info().Msg("-----< Message Queue Testing >-----")
	logger.Info().
		Str("groupName", groupName).
		Str("streamName", streamName).
		Str("consumerName", *consumerName).
		Str("action", *action).
		Any("samples", *samples).
		Any("errors", *errors).
		Str("redisEndpoint", *redisEndpoint).
		Msg("")

	// create and connect
	maxlen := int64(50)
	tmq, err := mq.NewRedisGtrsQueue[CityType](ctx, *redisEndpoint, streamName, maxlen)
	if err != nil {
		logger.Fatal().Err(err).Msg("NewRedisGtrsClientStefan()")
	}
	if err := tmq.Connect(ctx); err != nil {
		logger.Fatal().Err(err).Msg("tmq.Connec()")
	}
	defer tmq.Close()

	if *action == "rm" {
		logger.Info().Msg("-----< Action:remove stale [rm] >-----")

		total, queued, stale, err := tmq.GetState(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("tmq.GetState()")
		}
		logger.Info().
			Any("s.tot", total).
			Any("s.queued", queued).
			Any("s.stale", stale).
			Msg("before")

		deleted, err := tmq.DeleteStale(ctx, groupName, int64(*samples), 5*time.Minute)
		if err != nil {
			logger.Fatal().Err(err).Msg("tmq.DeleteStale()")
		}
		total, queued, stale, err = tmq.GetState(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("tmq.GetState()")
		}
		logger.Info().
			Any("s.tot", total).
			Any("s.queued", queued).
			Any("s.stale", stale).
			Any("x.deleted", deleted).
			Msg("after ")
	} else if *action == "tx" {
		logger.Info().Msg("-----< Action:send [tx] >-----")

		batch := 0
		k := 0
		for {
			batch += 1
			if k >= *samples {
				break
			}

			for _, name := range cityList {
				k += 1
				if k > *samples {
					break
				}
				city := CityType{Id: k, Created: time.Now(), Name: name}

				// trigger errors
				if *errors > 0 {
					if k%*errors == 0 {
						city.Trigger = 1
					}
				}

				// send
				id, err := tmq.Publish(ctx, city)
				if err != nil {
					logger.Fatal().Err(err).Msg("tmq.Publish()")
				}
				// get state
				total, queued, unconfirmed, err := tmq.GetState(ctx)
				if err != nil {
					logger.Fatal().Err(err).Msg("tmq.GetState()")
				}
				logger.Info().
					Any("sleep[ms]", *sleep/1e6).
					Str("item", fmt.Sprintf("%v.%v", batch, k)).
					Any("s.tot", total).
					Any("s.queued", queued).
					Any("s.unconf", unconfirmed).
					Any("m.id", id).
					Any("c.id", city.Id).
					Str("c.name", city.Name).Msg("")

				time.Sleep(*sleep)
			}
		}
	} else if *action == "rx" {
		logger.Info().Msg("-----< Action:receive [rx] >-----")

		if *consumerName == "" {
			logger.Fatal().Msg("missing consumerName")
		}

		for _, mode := range []string{">"} {
			logger.Info().Str("mode", mode).Msg("stream read mode")
			count := 0
			err = tmq.Subscribe(ctx, groupName, *consumerName, mode, func(msg gtrs.Message[CityType]) error {
				city := msg.Data
				delta := time.Since(city.Created)
				count += 1

				logger.Info().
					Any("sleep[ms]", *sleep/1e6).
					Any("count", count).
					Any("msg.id", msg.ID).
					Any("city.id", city.Id).
					Any("ago", delta.String()).
					Any("trigger", city.Trigger).
					Str("name", city.Name).Msg("")

				if city.Trigger > 0 {
					err := fmt.Errorf("error in in %s", city.Name)
					logger.Error().Err(err).Msg("FAIL")
					return err
				}

				time.Sleep(*sleep)
				return nil
			})
			if err != nil {
				logger.Fatal().Err(err).Msg("ReceiveResponseStefan()")
			}
		}
	} else {
		logger.Fatal().Msg("invalid action")
	}
	logger.Info().
		Str("time", time.Now().String()).
		Msg("done")
}
