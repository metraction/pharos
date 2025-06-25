package mq

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/metraction/pharos/internal/utils"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

// test worker queue against requirements:
//
// OK R1: Multiple workers can subscribe to the queue
// OK R2: Tasks are distributed amongst free workers (a task is given to exactly one worker)
//    R3: Priority 2 tasks are only served when there are no pending priority 1 tasks
// OK R4: A worker acknowledges when a task was completed successfully
// OK R5: Unacknowledged tasks are reassigned after a given time X
//    R6: No tasks are lost upon worker or backend crashes
// OK R7: Tasks are removed from queue after N unsuccessful executions by worker (tasks that systematically break)

var cityList = []string{"London", "Paris", "Berlin", "Madrid", "Rome", "Vienna", "Budapest", "Prague", "Warsaw", "Bucharest", "Barcelona", "Munich", "Milan", "Hamburg", "Brussels", "Amsterdam", "Lisbon", "Stockholm", "Copenhagen", "Dublin", "Athens", "Helsinki", "Oslo", "Zurich", "Geneva", "Frankfurt", "Lyon", "Naples", "Turin", "Seville", "Valencia", "Stuttgart", "Düsseldorf", "Dortmund", "Essen", "Leipzig", "Bremen", "Dresden", "Hanover", "Nuremberg", "Duisburg", "Glasgow", "Birmingham", "Manchester", "Liverpool", "Edinburgh", "Sheffield", "Bristol", "Leeds", "Nottingham", "Leicester", "Newcastle", "Cardiff", "Belfast", "Sofia", "Belgrade", "Zagreb", "Ljubljana", "Bratislava", "Tallinn", "Riga", "Vilnius", "Luxembourg", "Monaco", "San Marino", "Andorra la Vella", "Vaduz", "Reykjavik", "Tirana", "Skopje", "Podgorica", "Pristina", "Sarajevo", "Split", "Dubrovnik", "Krakow", "Gdansk", "Poznan", "Wroclaw", "Katowice", "Lodz", "Szczecin", "Bydgoszcz", "Lublin", "Bialystok", "Plovdiv", "Varna", "Burgas", "Constanta", "Cluj-Napoca", "Timisoara", "Iasi", "Brasov", "Galati", "Ploiesti", "Oradea", "Chisinau", "St. Petersburg", "Moscow", "Kazan", "Nizhny Novgorod", "Samara", "Rostov-on-Don", "Ufa", "Volgograd", "Perm", "Krasnodar", "Saratov", "Voronezh", "Krasnoyarsk", "Sofia", "Thessaloniki", "Patras", "Heraklion", "Larissa", "Volos", "Chania", "Ioannina", "Kavala", "Trikala", "Piraeus", "Antwerp", "New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Philadelphia", "San Antonio", "San Diego", "Dallas", "San Jose", "Austin", "Jacksonville", "Fort Worth", "Columbus", "Charlotte", "San Francisco", "Indianapolis", "Seattle", "Denver", "Washington", "Boston", "El Paso", "Nashville", "Detroit", "Oklahoma City", "Portland", "Las Vegas", "Memphis", "Louisville", "Baltimore", "Milwaukee", "Albuquerque", "Tucson", "Fresno", "Sacramento", "Mesa", "Kansas City", "Atlanta", "Omaha", "Colorado Springs", "Raleigh", "Miami", "Long Beach", "Virginia Beach", "Oakland", "Minneapolis", "Tulsa", "Arlington", "Tampa", "New Orleans", "Wichita", "Cleveland", "Bakersfield", "Aurora", "Anaheim", "Honolulu", "Santa Ana", "Riverside", "Corpus Christi", "Lexington", "Henderson", "Stockton", "Saint Paul", "Cincinnati", "St. Louis", "Pittsburgh", "Greensboro", "Lincoln", "Anchorage", "Plano", "Orlando"}
var consumerList = []string{"alfa", "bravo", "charlie", "delta"} //, "echo", "foxtrot"}

type CityType struct {
	Id      int
	Name    string
	Created time.Time
	Trigger int // trigger error
}

var MaxTime = time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)

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

// Helper: return list bachSize city names (to generate message samples)
func createSamples(batchSize int) []string {
	// repeat cityList until it has at least batchSize elements
	samples := lo.Flatten(lo.RepeatBy(batchSize/len(cityList)+2, func(_ int) []string { return cityList }))
	return lo.Slice(samples, 0, batchSize)
}

// Helper: generate worker specific delay (test fair distribution of tasks)
func workerProcessTime(k int, baseDelay time.Duration) time.Duration {
	return baseDelay * time.Duration(k+1) // cunsumer specific work time
}

// Helper: create and run a number of consumers, let them read messages or timeout
func runConsumers(ctx context.Context, tmq *RedisTaskQueue[CityType], consumerList []string, groupName string, taskErrorRate int, baseDelay, simulationTimeout time.Duration) (*utils.SafeIntMap, *utils.SafeIntMap, *utils.SafeTimeMap) {

	var wg sync.WaitGroup

	runCount := utils.NewSafeIntMap()
	runError := utils.NewSafeIntMap()
	runTime := utils.NewSafeTimeMap()

	fmt.Printf("create %v receivers with timeout %v sec, worker base delay %v sec:\n", consumerList, simulationTimeout.Seconds(), baseDelay.Seconds())

	wg.Add(len(consumerList))
	for k, consumerName := range consumerList {
		workTime := workerProcessTime(k+1, baseDelay)

		handlerFunc := cityHandlerFactory(consumerName, taskErrorRate, workTime, runCount, runError, runTime)
		go func() {
			defer wg.Done()
			fmt.Printf("- start GroupSubscribe[%s] ..\n", consumerName)
			err := tmq.GroupSubscribe(ctx, ">", groupName, consumerName, simulationTimeout, handlerFunc)
			fmt.Printf("- exit  GroupSubscribe[%s] with: %v\n", consumerName, err)
		}()
	}
	wg.Wait()
	fmt.Printf("done %v receivers\n", len(consumerList))
	return runCount, runError, runTime
}

// handler to process cities
func cityHandlerFactory(consumer string, retryOk int, workTime time.Duration, runCount *utils.SafeIntMap, runError *utils.SafeIntMap, runTime *utils.SafeTimeMap) TaskHandlerFunc[CityType] {

	handlerFunc := func(x TaskMessage[CityType]) error {
		count := runCount.Inc(consumer)

		fmt.Printf("RX[%-8s] %-16s | %v | sleep:%vs, trigger:%v, retry:%v, idle:%vs\n", consumer, x.Data.Name, count, workTime.Seconds(), x.Data.Trigger, x.RetryCount, x.IdleTime.Seconds())
		time.Sleep(workTime)

		runTime.Set(consumer, time.Now()) // record last processing time

		// simulate OK after one retry
		if x.RetryCount >= int64(retryOk) {
			return nil
		}
		// simulate permanent error
		if x.Data.Trigger == 1 {
			runError.Inc(consumer)
			return fmt.Errorf("scan failed (error)")
		}
		return nil
	}
	return handlerFunc
}

func TestWorkerQueue(t *testing.T) {

	// get redis or miniredis endpoint
	redisEndpoint, useMiniRedis := setupRedis(t)

	fmt.Println("-----< TEST SETUP >-----")
	fmt.Printf("redisEndpoint: %s (use miniredis:%v)\n", redisEndpoint, useMiniRedis)

	ctx := context.Background()

	// stream config and dimension
	streamName := "mx"
	groupName := "gx"
	maxStreamLen := int64(10000) // max # messages in stream

	// message limitations
	maxMsgRetry := int64(3)       // delete tasks after this many retries
	maxMsgTTL := 45 * time.Second // tasks idle longer than this are deleted upon cleanup/reclaim

	// cleanup process
	reclaimOlderThan := 1 * time.Second // get and reclaim messages with idleTime > this

	// experiment size
	batchSize := 1000
	testList := createSamples(batchSize) // ensure we have at least maxStreamLen cities for testing

	assert.Equal(t, batchSize, len(testList))

	var err error
	var tmq *RedisTaskQueue[CityType]

	// setup redis task queue
	tmq, err = NewRedisTaskQueue[CityType](ctx, redisEndpoint, streamName, maxStreamLen, maxMsgRetry, maxMsgTTL)
	assert.NoError(t, err)
	err = tmq.Connect(ctx)
	assert.NoError(t, err)

	err = tmq.CreateGroup(ctx, groupName, "$") // ensure the stream is created with defined settings if it not exists
	assert.NoError(t, err)

	// -----< STEP-1 Ensure initial state is empty >-----

	total, queued, stale, err := tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 0, int(total))
	assert.Equal(t, 0, int(queued))
	assert.Equal(t, 0, int(stale))

	// -----< STEP-2 Submit batchSize tasks >-----

	taskErrorRate := 5 // every x message throws error
	for k, name := range testList {
		city := CityType{
			Id:      k + 1,
			Created: time.Now(),
			Name:    name,
			Trigger: lo.Ternary(k%taskErrorRate == 0, 1, 0), // simulate error
		}
		_, err := tmq.AddMessage(ctx, 1, city) // submit task
		assert.NoError(t, err)
	}
	// ensure all tasks are in the queue
	total, queued, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, batchSize, int(total))
	assert.Equal(t, batchSize, int(queued))
	assert.Equal(t, 0, int(stale))

	// -----< STEP-3 receive tasks with multiple consumers >-----

	baseDelay := 10 * time.Millisecond    // worker processing time increment
	simulationTimeout := reclaimOlderThan // exit when no message arrives in this time (use reclaimOlderThan to ensure we can later reclaim stale tasks)

	// OK R1: Multiple workers can subscribe to the queue
	runCounts, runErrors, runTimes := runConsumers(ctx, tmq, consumerList, groupName, taskErrorRate, baseDelay, simulationTimeout)

	fmt.Printf("--< PUB/SUB %v tasks -->\n", batchSize)

	// get first and last execution time over all workers
	minTime := lo.MinBy(lo.Values(runTimes.Value), func(x, y time.Time) bool { return x.Before(y) })
	maxTime := lo.MinBy(lo.Values(runTimes.Value), func(x, y time.Time) bool { return !x.Before(y) })
	fmt.Printf("total tasks: count: %v, errors: %v, delta(first/last) %vsec\n", runCounts.Sum(), runErrors.Sum(), maxTime.Sub(minTime).Seconds())
	fmt.Printf("Worker\tTasks\tErrors\tSleep\n")
	for k, key := range consumerList {
		count := runCounts.Value[key]
		simerr := runErrors.Value[key]
		delay := workerProcessTime(k+1, baseDelay)

		fmt.Printf("%s\t%v\t%v\t%vms\n", key, count, simerr, delay.Milliseconds())
	}

	// OK R2: Tasks are distributed amongst free workers (a task is given to exactly one worker)
	//   ensure the workers finsish their last task within a small time difference, this indicates effective distribution of tasks
	assert.LessOrEqual(t, maxTime.Sub(minTime), 1*time.Second)

	// OK R4: A worker acknowledges when a task was completed successfully
	total, queued, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, batchSize, int(total), "stream total")         // R2 all tasks received in redis
	assert.Equal(t, int64(runErrors.Sum()), stale, "stream stale") // R4 all simulated error tasks are stale

	// diff between miniRedis and Redis: known issue that group.Lag in redis might be updated with delay
	if useMiniRedis {
		assert.Equal(t, batchSize, int(queued), "stream queued")
	} else {
		assert.Equal(t, 0, int(queued), "stream queued")
	}

	// OK R5: Unacknowledged tasks are reassigned after a given time X
	var num int64
	reclaimBlock1 := int64(5)
	num, err = tmq.ReclaimStale(ctx, groupName, consumerList[0], reclaimBlock1, reclaimOlderThan, func(x TaskMessage[CityType]) error {
		return nil // simulate successfull task execution
	})
	assert.NoError(t, err)
	assert.Equal(t, reclaimBlock1, num)

	total, _, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, batchSize, int(total), "stream total")
	assert.Equal(t, int64(runErrors.Sum())-reclaimBlock1, int64(stale), "stream stale") // R5

	//    R7 Tasks are removed from queue after N unsuccessful executions by worker (tasks that systematically break)
	reclaimBlock2 := int64(2)
	reclaimOlderThan = 0 * time.Second
	rounds := int64(3)
	for k := int64(1); k <= rounds; k++ {
		num, err = tmq.ReclaimStale(ctx, groupName, consumerList[0], reclaimBlock2, reclaimOlderThan, func(x TaskMessage[CityType]) error {
			assert.LessOrEqual(t, x.RetryCount, maxMsgRetry) // no reclaimed task has RetryCount > max
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, reclaimBlock2, num)
	}

	total, _, stale, err = tmq.GetState(ctx)
	assert.NoError(t, err)
	assert.Equal(t, batchSize, int(total), "stream total")
	assert.Equal(t, int64(runErrors.Sum())-reclaimBlock1-rounds*reclaimBlock2, int64(stale), "stream stale") // R7

	// Delete reminder of tasks
	assert.Greater(t, stale, int64(0)) // ensure we have stale items before deletion
	num, err = tmq.RemoveStale(ctx, groupName, int64(batchSize), reclaimOlderThan)
	assert.NoError(t, err)
	assert.Greater(t, num, int64(0)) // ensure deleted some

	num, err = tmq.RemoveStale(ctx, groupName, int64(batchSize), reclaimOlderThan)
	assert.NoError(t, err)
	assert.Equal(t, num, int64(0)) // ensure no more to delete

	fmt.Println("done")

}
