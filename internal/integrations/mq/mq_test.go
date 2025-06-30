package mq

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	Trigger int            // trigger error
	Context map[string]any // complex structure test
}
type CityResultType struct {
	Cid        int
	Name       string
	ResultName string
	Context    map[string]any
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

func showGroupStats(title string, stats GroupStats) {
	fmt.Printf("stats:%-15v read:%v, pending:%v, lag:%v, pressure:%v in %v\n", title, stats.Read, stats.Pending, stats.Lag, stats.BackPressureOr(0), stats.Groups)
}
func TestRedisWorkerGroup(t *testing.T) {

	// get redis or miniredis endpoint
	redisEndpoint, useMiniRedis := setupRedis(t)

	fmt.Println("-----< TEST SETUP >-----")
	fmt.Printf("redisEndpoint: %s (use miniredis:%v)\n", redisEndpoint, useMiniRedis)

	ctx := context.Background()

	// stream config and dimension
	maxLen := int64(1000)          // max # messages in stream (e.g. test backpressure)
	txStream := "scan-task-test"   // send scan tasks
	rxStream := "scan-result-test" // send scan results

	txMq, err := NewRedisWorkerGroup[CityTaskType](ctx, redisEndpoint, "$", txStream, "tasks", maxLen)
	assert.NoError(t, err)
	assert.NotNil(t, txMq)

	err = txMq.Connect(ctx)
	assert.NoError(t, err)
	defer txMq.Close()

	rxMq, err := NewRedisWorkerGroup[CityResultType](ctx, redisEndpoint, "$", rxStream, "results", maxLen)
	assert.NoError(t, err)
	assert.NotNil(t, rxMq)

	if err != nil {
		t.FailNow()
	}
	err = rxMq.Connect(ctx)
	assert.NoError(t, err)
	defer rxMq.Close()

	if err != nil {
		t.FailNow()
	}
	// ensure clean start
	assert.NoError(t, txMq.Delete(ctx))
	assert.NoError(t, rxMq.Delete(ctx))
	// create group
	assert.NoError(t, txMq.CreateGroup(ctx))
	assert.NoError(t, rxMq.CreateGroup(ctx))

	// publish N tasks
	samples := int(maxLen / 5)

	fmt.Printf("\n-----< SEND %v cities >-----\n", samples)

	stats, _ := txMq.GroupStats(ctx, "*")
	showGroupStats("before", stats)
	assert.Equal(t, int64(0), stats.Read)
	assert.Equal(t, int64(0), stats.Lag)
	assert.Equal(t, int64(0), stats.Pending)
	assert.Equal(t, float64(0), stats.BackPressureOr(-1))

	errorMsgs := 0
	for k, name := range createSamples(samples) {
		city := CityTaskType{Cid: k, Name: name, Created: time.Now(), Context: ContextGenerator()}
		if k%3 == 0 {
			errorMsgs += 1
			city.Name = city.Name + " (ERR)"
		}
		id, err := txMq.Publish(ctx, 1, city)
		//fmt.Println(">>", txMq.StreamName, id, city.Cid, city.Name)
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	}

	stats, _ = txMq.GroupStats(ctx, "*")
	showGroupStats(fmt.Sprintf("sent-%v", samples), stats)

	assert.Equal(t, int64(0), stats.Read)
	assert.Equal(t, int64(samples), stats.Lag)
	assert.Equal(t, int64(0), stats.Pending)
	assert.Greater(t, stats.BackPressureOr(-1), float64(0))

	// subscribe scan-t
	taskHandler := func(x TaskMessage[CityTaskType]) error {
		result := CityResultType{Cid: x.Data.Cid, Name: strings.ToUpper(x.Data.Name)}

		if x.RetryCount > 2 {
			fmt.Println("<< ", x.StreamName, x.GroupName, x.RetryCount, x.Id, "ack and forget")
			rxMq.Publish(ctx, 1, result)
			return nil
		} else if strings.Contains(x.Data.Name, "(ERR)") {
			return fmt.Errorf("ERR %v %v %v %v %v %v", x.StreamName, x.GroupName, x.RetryCount, x.Id, x.Data.Cid, x.Data.Name)
		}
		//fmt.Println("<< ", x.StreamName, x.GroupName, x.RetryCount, x.Id, x.Data.Cid, x.Data.Name)

		rxMq.Publish(ctx, 1, result)

		return nil
	}

	fmt.Printf("\n-----< SUBSCRIBE >-----\n")

	claimBlock := int64(maxLen / 2)
	claimMinIdle := 1 * time.Second // reclaim Non-ACK messages after 1 sec
	blockTime := 1 * time.Second    // block/wait for XReadGroup
	runTimeout := 2 * time.Second

	stats1, _ := txMq.GroupStats(ctx, "*")

	txMq.Subscribe(ctx, "alfa", claimBlock, claimMinIdle, blockTime, runTimeout, taskHandler)

	stats2, _ := txMq.GroupStats(ctx, "*")
	showGroupStats("subs-before", stats1)
	showGroupStats("subs-after", stats2)

	// miniredis reports different lag
	if !useMiniRedis {
		assert.Equal(t, int64(200), stats2.Read)
	}

	assert.Equal(t, int64(errorMsgs), stats2.Pending)

	//assert.True(t, false)
}
