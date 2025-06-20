package mq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/metraction/pharos/internal/utils"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

var cityList = []string{"London", "Paris", "Berlin", "Madrid", "Rome", "Vienna", "Budapest", "Prague", "Warsaw", "Bucharest", "Barcelona", "Munich", "Milan", "Hamburg", "Brussels", "Amsterdam", "Lisbon", "Stockholm", "Copenhagen", "Dublin", "Athens", "Helsinki", "Oslo", "Zurich", "Geneva", "Frankfurt", "Lyon", "Naples", "Turin", "Seville", "Valencia", "Stuttgart", "Düsseldorf", "Dortmund", "Essen", "Leipzig", "Bremen", "Dresden", "Hanover", "Nuremberg", "Duisburg", "Glasgow", "Birmingham", "Manchester", "Liverpool", "Edinburgh", "Sheffield", "Bristol", "Leeds", "Nottingham", "Leicester", "Newcastle", "Cardiff", "Belfast", "Sofia", "Belgrade", "Zagreb", "Ljubljana", "Bratislava", "Tallinn", "Riga", "Vilnius", "Luxembourg", "Monaco", "San Marino", "Andorra la Vella", "Vaduz", "Reykjavik", "Tirana", "Skopje", "Podgorica", "Pristina", "Sarajevo", "Split", "Dubrovnik", "Krakow", "Gdansk", "Poznan", "Wroclaw", "Katowice", "Lodz", "Szczecin", "Bydgoszcz", "Lublin", "Bialystok", "Plovdiv", "Varna", "Burgas", "Constanta", "Cluj-Napoca", "Timisoara", "Iasi", "Brasov", "Galati", "Ploiesti", "Oradea", "Chisinau", "St. Petersburg", "Moscow", "Kazan", "Nizhny Novgorod", "Samara", "Rostov-on-Don", "Ufa", "Volgograd", "Perm", "Krasnodar", "Saratov", "Voronezh", "Krasnoyarsk", "Sofia", "Thessaloniki", "Patras", "Heraklion", "Larissa", "Volos", "Chania", "Ioannina", "Kavala", "Trikala", "Piraeus", "Antwerp", "New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte", "San Francisco", "Indianapolis", "Seattle", "Denver", "Washington", "Boston", "El Paso", "Nashville", "Detroit", "Oklahoma City", "Portland", "Las Vegas", "Memphis", "Louisville", "Baltimore", "Milwaukee", "Albuquerque", "Tucson", "Fresno", "Sacramento", "Mesa", "Kansas City", "Atlanta", "Omaha", "Colorado Springs", "Raleigh", "Miami", "Long Beach", "Virginia Beach", "Oakland", "Minneapolis", "Tulsa", "Arlington", "Tampa", "New Orleans", "Wichita", "Cleveland", "Bakersfield", "Aurora", "Anaheim", "Honolulu", "Santa Ana", "Riverside", "Corpus Christi", "Lexington", "Henderson", "Stockton", "Saint Paul", "Cincinnati", "St. Louis", "Pittsburgh", "Greensboro", "Lincoln", "Anchorage", "Plano", "Orlando"}
var consumerList = []string{"alfa", "bravo"} //, "charlie", "delta", "echo", "foxtrot"}

type CityType struct {
	Id      int
	Created time.Time
	Name    string
	Trigger int
}

var MaxTime = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)

// test worker queue against requirements
// OK R1: Multiple workers can subscribe to the queue
// OK R2: Tasks are distributed amongst free workers (a task is given to exactly one worker)
// R3 Priority 2 tasks are only served when there are no pending priority 1 tasks
// OK R4: A worker acknowledges when a task was completed successfully
// R5 Unacknowledged tasks are reassigned after a given time X
// R6 No tasks are lost upon worker or backend crashes
// R7 Tasks are removed from queue after N unsuccessful executions by worker (tasks that systematically break)

func TestWorkerQueue(t *testing.T) {

	// prepare local redis
	mr, err := miniredis.Run()
	assert.NoError(t, err)

	// select redis instance for tests
	redisEndpoint := os.Getenv("TEST_REDIS_ENDPOINT")
	useMiniRedis := lo.Ternary(redisEndpoint == "", true, false) // some results differ
	redisEndpoint = lo.Ternary(redisEndpoint == "", "redis://"+mr.Addr(), redisEndpoint)

	fmt.Println("redisEndpoint", redisEndpoint)

	ctx := context.Background()

	streamName := "mq"
	groupName := "scan"

	timeout := 1 * time.Second // exit listener when no new message arrives in this time
	var batchSize int = 100
	var maxStreamLen int64 = 100
	var errorRate int = 5  // 1 in 20 messages trigger error in handler
	var maxRetry int64 = 1 // delete messages after maxRetry
	var maxMsgTTL time.Duration = 1 * time.Second
	var elapsedBase = 5 // worker delay increment in ms between consumers

	// create TaskQueue
	tmq, err := NewRedisGtrsQueue[CityType](ctx, redisEndpoint, streamName, maxStreamLen, maxRetry, maxMsgTTL)
	assert.NoError(t, err)

	// try connect
	assert.NoError(t, tmq.Connect(ctx))
	defer tmq.Close()

	// create group
	assert.NoError(t, tmq.CreateGroup(ctx, groupName, "$"))

	// ensure initial state is empty
	total, queued, stale, err := tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, int(total))
	assert.Equal(t, 0, int(queued))
	assert.Equal(t, 0, int(stale))

	// repeat cityList until it has at least batchSize elements
	testList := lo.Flatten(lo.RepeatBy(batchSize/len(cityList)+2, func(_ int) []string { return cityList }))

	// publish batchSize messages
	for k, name := range testList {
		if k >= batchSize {
			break
		}
		city := CityType{Id: k + 1, Created: time.Now(), Name: name}
		// used to simulate errors in handler function
		if k%errorRate == 0 {
			city.Trigger = 1
		}
		_, err := tmq.Publish(ctx, city)
		assert.NoError(t, err)
	}

	// check state
	total, queued, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, batchSize, int(total))
	assert.Equal(t, batchSize, int(queued))
	assert.Equal(t, 0, int(stale))

	// read messages
	var count int64 = 0
	var errors int64 = 0
	var wg sync.WaitGroup

	wg.Add(len(consumerList))
	consumerTime := utils.NewSafeTimeMap()
	consumerCount := utils.NewSafeIntMap()

	// cenerate worker specific processing delay
	workDelay := func(k int) time.Duration {
		return time.Duration(k) * time.Duration(elapsedBase) * time.Millisecond
	}

	// Verify: R1 Multiple workers can subscribe to the queue
	for k, consumerName := range consumerList {
		go func() {
			defer wg.Done()
			fmt.Printf("create [%s]\n", consumerName)
			err = tmq.Subscribe(ctx, groupName, consumerName, ">", timeout, func(msg TaskMessage[CityType]) error {
				atomic.AddInt64(&count, 1)
				consumerCount.Inc(consumerName)
				city := msg.Data
				if city.Trigger > 0 {
					atomic.AddInt64(&errors, 1)
					return fmt.Errorf("error[%s] in %s", consumerName, city.Name)
				}
				// simulate work delay
				time.Sleep(workDelay(k + 1))
				consumerTime.Set(consumerName, time.Now()) // record last processing time
				return nil
			})
			fmt.Printf("drop [%s]: %v\n", consumerName, err)
		}()
	}
	wg.Wait()

	fmt.Printf("--< PUB/SUB %v tasks -->\n", batchSize)
	// get first and last execution time over all workers
	minTime := lo.MinBy(lo.Values(consumerTime.Value), func(x, y time.Time) bool { return x.Before(y) })
	maxTime := lo.MinBy(lo.Values(consumerTime.Value), func(x, y time.Time) bool { return !x.Before(y) })
	fmt.Printf("count: %v, errors: %v, delta(first/last) %vsec\n", count, errors, maxTime.Sub(minTime).Seconds())
	for k, key := range consumerList {
		val := consumerCount.Value[key]
		fmt.Printf("%s\t%v\t%v ms\t%v\n", key, val, (k+1)*elapsedBase, (k+1)*elapsedBase*val)
	}

	// Verify R2: Tasks are distributed amongst free workers (a task is given to exactly one worker)
	//   ensure the workers finsish their last task within a small time difference
	assert.LessOrEqual(t, maxTime.Sub(minTime), workDelay(len(consumerList)+1)+time.Second)

	// Verify R2: All tasks were processed
	// Verify R4: A worker acknowledges when a task was completed successfully
	total, queued, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, batchSize, int(total), "stream total") // R2
	assert.Equal(t, errors, int64(stale), "stream stale")  // R4

	// diff between miniRedis and Redis, known issue that group.Lag in redis might be updated with delay
	if useMiniRedis {
		assert.Equal(t, batchSize, int(queued), "stream queued")
	} else {
		assert.Equal(t, 0, int(queued), "stream queued")
	}

	fmt.Printf("--< RECLAIM %v/%v tasks -->\n", errors, batchSize)

	// reclaim tasks
	println("before: tot/queued/stale", total, queued, stale)

	for _, consumer := range lo.Slice(consumerList, 0, 1) {
		fmt.Printf("reclaim %v stale tasks for %v\n", stale, consumer)
		count, err := tmq.Reclaim(ctx, groupName, consumer, 5, 0*time.Second)
		assert.NoError(t, err)
		assert.Greater(t, count, int64(0))
		fmt.Printf("reclaimed %v\n", count)

		// ensure reclaimed messages are deleted
		fmt.Println("process reclaimed")
		timeout := 1 * time.Second
		_ = tmq.Subscribe(ctx, groupName, consumer, "0", timeout, func(msg TaskMessage[CityType]) error {
			fmt.Printf("call: retry:%v idle:%v, id:%v\n", msg.RetryCount, msg.IdleTime.Seconds(), msg.MsgId)
			return nil
		})
		println("reclaim+subscribe done for 1", consumer)
	}

	total, queued, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	println(" after: tot/queued/stale", total, queued, stale)

	// delete stale 1: nothing deleted given the timeout
	deleted, err := tmq.DeleteStale(ctx, groupName, int64(batchSize), 5*time.Minute)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), deleted)

	// delete stale 2: all deleted
	timeout = 100 * time.Millisecond
	time.Sleep(timeout)

	deleted, err = tmq.DeleteStale(ctx, groupName, int64(batchSize), timeout)
	assert.NoError(t, err)
	assert.Equal(t, errors, deleted)

}
