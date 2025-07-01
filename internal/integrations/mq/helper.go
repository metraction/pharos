package mq

import (
	"math/rand"

	"github.com/samber/lo"
)

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
