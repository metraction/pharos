package mq

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

var cityList = []string{"London", "Paris", "Berlin", "Madrid", "Rome", "Vienna", "Budapest", "Prague", "Warsaw", "Bucharest", "Barcelona", "Munich", "Milan", "Hamburg", "Brussels", "Amsterdam", "Lisbon", "Stockholm", "Copenhagen", "Dublin", "Athens", "Helsinki", "Oslo", "Zurich", "Geneva", "Frankfurt", "Lyon", "Naples", "Turin", "Seville", "Valencia", "Stuttgart", "Düsseldorf", "Dortmund", "Essen", "Leipzig", "Bremen", "Dresden", "Hanover", "Nuremberg", "Duisburg", "Glasgow", "Birmingham", "Manchester", "Liverpool", "Edinburgh", "Sheffield", "Bristol", "Leeds", "Nottingham", "Leicester", "Newcastle", "Cardiff", "Belfast", "Sofia", "Belgrade", "Zagreb", "Ljubljana", "Bratislava", "Tallinn", "Riga", "Vilnius", "Luxembourg", "Monaco", "San Marino", "Andorra la Vella", "Vaduz", "Reykjavik", "Tirana", "Skopje", "Podgorica", "Pristina", "Sarajevo", "Split", "Dubrovnik", "Krakow", "Gdansk", "Poznan", "Wroclaw", "Katowice", "Lodz", "Szczecin", "Bydgoszcz", "Lublin", "Bialystok", "Plovdiv", "Varna", "Burgas", "Constanta", "Cluj-Napoca", "Timisoara", "Iasi", "Brasov", "Galati", "Ploiesti", "Oradea", "Chisinau", "St. Petersburg", "Moscow", "Kazan", "Nizhny Novgorod", "Samara", "Rostov-on-Don", "Ufa", "Volgograd", "Perm", "Krasnodar", "Saratov", "Voronezh", "Krasnoyarsk", "Sofia", "Thessaloniki", "Patras", "Heraklion", "Larissa", "Volos", "Chania", "Ioannina", "Kavala", "Trikala", "Piraeus", "Antwerp", "New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte", "San Francisco", "Indianapolis", "Seattle", "Denver", "Washington", "Boston", "El Paso", "Nashville", "Detroit", "Oklahoma City", "Portland", "Las Vegas", "Memphis", "Louisville", "Baltimore", "Milwaukee", "Albuquerque", "Tucson", "Fresno", "Sacramento", "Mesa", "Kansas City", "Atlanta", "Omaha", "Colorado Springs", "Raleigh", "Miami", "Long Beach", "Virginia Beach", "Oakland", "Minneapolis", "Tulsa", "Arlington", "Tampa", "New Orleans", "Wichita", "Cleveland", "Bakersfield", "Aurora", "Anaheim", "Honolulu", "Santa Ana", "Riverside", "Corpus Christi", "Lexington", "Henderson", "Stockton", "Saint Paul", "Cincinnati", "St. Louis", "Pittsburgh", "Greensboro", "Lincoln", "Anchorage", "Plano", "Orlando"}
var consumerList = []string{"alfa", "bravo", "charlie", "delta"} //, "echo", "foxtrot"}

// Helper: return list bachSize city names (to generate message samples)
// repeat cityList until it has at least batchSize elements
func createSamples(batchSize int) []string {

	samples := lo.Flatten(lo.RepeatBy(batchSize/len(cityList)+2, func(_ int) []string { return cityList }))
	return lo.Slice(samples, 0, batchSize)
}

type CityTaskType struct {
	Cid     int
	Name    string
	Created time.Time
	Trigger int // trigger error

}
type CityResultType struct {
	Cid        int
	Name       string
	ResultName string
}

// Helper: setup redis test endpoint (miniredis or external instance)
func setupRedis(t *testing.T) (string, bool) {

	// prepare local redis
	mr, err := miniredis.Run()
	assert.NoError(t, err)

	// select redis instance for tests
	redisEndpoint := os.Getenv("TEST_REDIS_ENDPOINT")
	useMiniRedis := lo.Ternary(redisEndpoint == "", true, false) // some results differ
	redisEndpoint = lo.Ternary(redisEndpoint == "", "redis://"+mr.Addr(), redisEndpoint)

	return redisEndpoint, useMiniRedis
}

func TestRedisWorkerGroup(t *testing.T) {

	// get redis or miniredis endpoint
	redisEndpoint, useMiniRedis := setupRedis(t)

	fmt.Println("-----< TEST SETUP >-----")
	fmt.Printf("redisEndpoint: %s (use miniredis:%v)\n", redisEndpoint, useMiniRedis)

	ctx := context.Background()

	// stream config and dimension

	txStream := "scan-t"  // send scan tasks
	rxStream := "scan-r"  // send scan results
	maxLen := int64(1000) // max # messages in stream

	txMq, err := NewRedisWorkerGroup[CityTaskType](ctx, redisEndpoint, "$", txStream, "scanner", maxLen)
	assert.NoError(t, err)
	assert.NotNil(t, txMq)

	err = txMq.Connect(ctx)
	assert.NoError(t, err)
	defer txMq.Close()

	rxMq, err := NewRedisWorkerGroup[CityResultType](ctx, redisEndpoint, "$", rxStream, "controller", maxLen)
	assert.NoError(t, err)
	assert.NotNil(t, rxMq)
	err = rxMq.Connect(ctx)
	assert.NoError(t, err)
	defer rxMq.Close()

	// publish N tasks
	samples := 0
	for k, name := range createSamples(samples) {
		city := CityTaskType{Cid: k, Name: name, Created: time.Now()}
		id, err := txMq.Publish(ctx, 1, city)
		fmt.Println(id, city.Cid, city.Name)
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	}

	// subscribe scan-t
	taskHandler := func(x TaskMessage[CityTaskType]) error {
		fmt.Printf("rx-t | %-4v | %-10s |\n", x.Data.Cid, x.Data.Name)
		if x.Data.Cid%3 == 0 {
			return fmt.Errorf("sim error at %v for %s", x.Data.Cid, x.Data.Name)
		}
		return nil
	}

	pendingBlock := int64(10)
	err = txMq.Subscribe(ctx, "alfa", pendingBlock, 1*time.Second, taskHandler)

	fmt.Println("err", err)

	//assert.True(t, false)
}
