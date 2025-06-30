package mq

import (
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"github.com/metraction/pharos/internal/utils"
	"github.com/samber/lo"
)

// return parts from task queue DSN "queue://stream:group/?maxlen=1000&maxretry=2&maxttl=1h"
func ParseTaskQueueDsn(input string) (string, string, int64, int64, time.Duration, error) {

	var streamName string
	var groupName string
	var maxLen int64
	var maxRetry int64
	var maxTTL time.Duration

	rex1 := regexp.MustCompile(`queue://([^:]+):([^:/]+).*`)
	if match := rex1.FindStringSubmatch(input); len(match) > 1 {
		streamName = match[1]
		groupName = match[2]
	} else {
		return "", "", 0, 0, maxTTL, fmt.Errorf("invalid DSN (1) %v", input)
	}

	rex2 := regexp.MustCompile(`([?&])([^=&]+)=([^&]+)`)
	for _, match := range rex2.FindAllStringSubmatch(input, -1) {
		// match[2] is the key, match[3] is the value
		key := match[2]
		val := match[3]
		if key == "maxlen" {
			maxLen = utils.ToNumOr[int64](val, 0)
		}
		if key == "maxretry" {
			maxRetry = utils.ToNumOr[int64](val, 0)
		}
		if key == "maxttl" {
			maxTTL, _ = time.ParseDuration(val)
		}

	}
	return streamName, groupName, maxLen, maxRetry, maxTTL, nil
}

// generate related contexts
func ContextGenerator() map[string]any {

	ctxt := map[string]any{}
	clusters := []string{"dev", "txt", "pre", "mte", "prd"}
	owners := []string{"Darth Vader", "Luke Skywalker", "Han Solo", "Princess Leia", "Yoda", "Obi-Wan Kenobi", "R2-D2", "C-3PO", "Chewbacca", "Din Djarin"}
	cities := map[string][]string{
		"Abruzzo":        []string{"Pescara", "Aquila", "Teramo"},
		"AostaValley":    []string{"Aosta", "Saint-Vincent", "Sarre"},
		"Apulia":         []string{"Bari", "Taranto", "Foggia"},
		"Basilicata":     []string{"Potenza", "Matera", "Melfi"},
		"Calabria":       []string{"ReggioCalabria", "Catanzaro", "Lamezia Terme"},
		"Campania":       []string{"Naples", "Salerno", "Giugliano in Campania"},
		"Emilia-Romagna": []string{"Bologna", "Parma", "Modena"},
		"Friuli-Venezia": []string{"Trieste", "Udine", "Pordenone"},
		"Lazio":          []string{"Rome", "Latina", "Guidonia Montecelio"},
		"Liguria":        []string{"Genoa", "LaSpezia", "Savona"},
		"Lombardy":       []string{"Milan", "Brescia", "Monza"},
		"Marche":         []string{"Ancona", "Pesaro", "Fano"},
		"Molise":         []string{"Campobasso", "Termoli", "Isernia"},
		"Piedmont":       []string{"Turin", "Novara", "Alessandria"},
		"Sardinia":       []string{"Cagliari", "Sassari", "Quartu SantElena"},
		"Sicily":         []string{"Palermo", "Catania", "Messina"},
		"Trentino-Alto":  []string{"Bolzano", "Trento", "Merano"},
		"Tuscany":        []string{"Florence", "Prato", "Livorno"},
		"Umbria":         []string{"Perugia", "Terni", "Foligno"},
		"Veneto":         []string{"Venice", "Verona", "Padua"}}

	namespaces := lo.Keys(cities)
	namespace := lo.Sample(namespaces)

	ctxt["cluster"] = lo.Sample(clusters)
	ctxt["namespace"] = namespace
	ctxt["apps"] = lo.Samples(cities[namespace], rand.Intn(3)) // add N apps related to namespace
	ctxt["owner"] = lo.Sample(owners)
	return ctxt
}
