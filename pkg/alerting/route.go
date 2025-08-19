package alerting

import (
	"fmt"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

type Route struct {
	RouteConfig    *model.RouteConfig
	Receiver       *Receiver
	Alerts         []*model.Alert
	AlertGroups    map[string]*AlertGroup
	Logger         *zerolog.Logger
	RepeatInterval time.Duration
	Path           string // Path to the route, used for storing data about when an alert was sent the last time.
	// B-Tree structure
	FirstChild  *Route
	NextSibling *Route
}

func NewRoute(routeConfig *model.RouteConfig, alertingConfig *model.AlertingConfig, path string) *Route {
	if path == "" {
		path = "root"
	}
	// find receiver in alertingConfig.Receivers
	var receiver *Receiver
	for _, r := range alertingConfig.Receivers {
		if r.Name == routeConfig.Receiver {
			receiver = NewReceiver(&r)
			break
		}
	}

	r := &Route{
		RouteConfig: routeConfig,
		Receiver:    receiver,
		Alerts:      []*model.Alert{},
		Logger:      logging.NewLogger("info", "component", fmt.Sprintf("Route %s", path)),
		Path:        path,
	}
	if receiver == nil {
		r.Logger.Panic().Str("receiver", routeConfig.Receiver).Msg("Receiver not found")
	}
	// Initialize some defaults for receiver, groupwait and groupinterval
	if routeConfig.Receiver == "" {
		routeConfig.Receiver = "default"
	}
	if routeConfig.GroupInterval == "" {
		routeConfig.GroupInterval = "5m"
	}
	if routeConfig.GroupWait == "" {
		routeConfig.GroupWait = "30s"
	}
	if routeConfig.RepeatInterval == "" {
		routeConfig.RepeatInterval = "4h"
	}
	repeatInterval, err := time.ParseDuration(routeConfig.RepeatInterval)
	if err != nil {
		r.Logger.Fatal().Err(err).Msg("Failed to parse repeat interval")
	}
	r.RepeatInterval = repeatInterval
	// Handle child
	if len(routeConfig.ChildRoutes) > 0 {
		r.FirstChild = NewRoute(r.GetRouteConfigForChild(routeConfig.ChildRoutes[0]), alertingConfig, path+"[0]")
	}
	// Handle siblings of child
	if len(routeConfig.ChildRoutes) > 1 {
		current := r.FirstChild
		for i := 1; i < len(routeConfig.ChildRoutes); i++ {
			next := NewRoute(r.GetRouteConfigForChild(routeConfig.ChildRoutes[i]), alertingConfig, fmt.Sprintf("%s[%d]", path, i))
			current.NextSibling = next
			current = next
		}
	}
	return r
}

func (r *Route) String() string {
	return r.Path
}

func (r *Route) UpdateAlertGroups() {
	if r.AlertGroups == nil {
		r.AlertGroups = make(map[string]*AlertGroup)
	}
	for _, alert := range r.Alerts {
		groupLabels := make(map[string]string)
		for _, groupBy := range r.RouteConfig.GroupBy {
			for _, label := range alert.Labels {
				if label.Name == groupBy {
					groupLabels[groupBy] = label.Value
				}
			}
		}
		groupKey := getGroupKey(groupLabels)
		// Check if the group already exists
		if _, exists := r.AlertGroups[groupKey]; !exists {
			r.AlertGroups[groupKey] = NewAlertGroup(r.RouteConfig, groupLabels)
		} else {
			// clear old alerts
			r.AlertGroups[groupKey].Alerts = []model.Alert{}
		}
		r.AlertGroups[groupKey].Alerts = append(r.AlertGroups[groupKey].Alerts, *alert)
		r.Logger.Debug().Str("groupKey", groupKey).Msg("Updating alert group")
	}
	for groupkey, group := range r.AlertGroups {
		r.Logger.Debug().Str("groupKey", groupkey).Msg("Sending alerts for group")
		r.Receiver.SendAlerts(group, r)
	}

}

func (r *Route) GetRouteConfigForChild(childRouteconfig model.RouteConfig) *model.RouteConfig {
	if childRouteconfig.Receiver == "" {
		childRouteconfig.Receiver = r.RouteConfig.Receiver
	}
	if childRouteconfig.GroupBy == nil {
		childRouteconfig.GroupBy = r.RouteConfig.GroupBy
	}
	if childRouteconfig.GroupInterval == "" {
		childRouteconfig.GroupInterval = r.RouteConfig.GroupInterval
	}
	if childRouteconfig.GroupWait == "" {
		childRouteconfig.GroupWait = r.RouteConfig.GroupWait
	}
	if childRouteconfig.RepeatInterval == "" {
		childRouteconfig.RepeatInterval = r.RouteConfig.RepeatInterval
	}
	return &childRouteconfig
}

func (r *Route) getMatchedAlerts(alerts []*model.Alert, invert bool) []*model.Alert {
	var matchedAlerts []*model.Alert
	var unmatchedAlerts []*model.Alert
	if len(r.RouteConfig.Matchers) == 0 {
		r.Logger.Info().Msg("No matchers defined, returning all alerts")
	}
	for _, alert := range alerts {
		matched := true
		for _, matcherString := range r.RouteConfig.Matchers {
			if matcher, err := NewMatcher(matcherString); err != nil {
				r.Logger.Error().Err(err).Msg("Failed to create matcher")
				continue
			} else {
				if !matcher.Matches(*alert) {
					matched = false
					break
				}
			}
		}
		if matched {
			matchedAlerts = append(matchedAlerts, alert)
		} else {
			unmatchedAlerts = append(unmatchedAlerts, alert)
		}
	}
	if invert {
		return unmatchedAlerts
	}
	r.Logger.Debug().Int("matched", len(matchedAlerts)).Int("unmatched", len(unmatchedAlerts)).Msg("Returning matched alerts")
	return matchedAlerts
}

func (r *Route) GetMatchedAlerts(alerts []*model.Alert) []*model.Alert {
	return r.getMatchedAlerts(alerts, false)
}

func (r *Route) GetUnmatchedAlerts(alerts []*model.Alert) []*model.Alert {
	return r.getMatchedAlerts(alerts, true)
}

func (r *Route) SendAlerts(alerts []*model.Alert) {
	r.Alerts = r.GetMatchedAlerts(alerts)
	r.UpdateAlertGroups()
	r.Logger.Info().Int("alerts", len(r.Alerts)).Msg("Sending alerts")
	if r.FirstChild != nil {
		r.FirstChild.SendAlerts(alerts)
	}
	if r.NextSibling != nil {
		r.NextSibling.SendAlerts(r.GetSiblingAlerts(alerts))
	}
}

func (r *Route) GetSiblingAlerts(alerts []*model.Alert) []*model.Alert {
	if r.RouteConfig.Continue {
		return alerts
	}
	return r.GetUnmatchedAlerts(alerts)
}
