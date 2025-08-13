package alerting

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"github.com/metraction/pharos/internal/logging"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

func GetPrometheusAlert(a *model.Alert) *model.PrometheusAlert {
	if a == nil {
		return nil
	}
	labels := make(map[string]string, len(a.Labels))
	for _, l := range a.Labels {
		labels[l.Name] = l.Value
	}
	annotations := make(map[string]string, len(a.Annotations))
	for _, a := range a.Annotations {
		annotations[a.Name] = a.Value
	}
	return &model.PrometheusAlert{
		Status:       a.Status,
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     a.StartsAt,
		EndsAt:       a.EndsAt,
		GeneratorURL: a.GeneratorURL,
		Fingerprint:  a.Fingerprint,
	}
}

func HandleAlerts(databaseContext *model.DatabaseContext) func(item model.PharosImageMeta) model.PharosImageMeta {
	return func(item model.PharosImageMeta) model.PharosImageMeta {
		for _, contextRoot := range item.ContextRoots {
			labels := []model.AlertLabel{}
			labels = append(labels, model.AlertLabel{
				Name:  "imagespec",
				Value: item.ImageSpec,
			})
			labels = append(labels, model.AlertLabel{
				Name:  "imageid",
				Value: item.ImageId,
			})
			labels = append(labels, model.AlertLabel{
				Name:  "digest",
				Value: item.ManifestDigest,
			})
			labels = append(labels, model.AlertLabel{
				Name:  "platform",
				Value: item.ArchOS + "/" + item.ArchName,
			})
			for _, context := range contextRoot.Contexts {
				for label, value := range context.Data {
					switch v := value.(type) {
					case string, int, int32, int64, float32, float64, bool, time.Time, time.Duration:
						labels = append(labels, model.AlertLabel{
							Name:  label,
							Value: fmt.Sprintf("%v", v),
						})
					default:
					}
				}
			}
			severities := item.GetSummary().Severities
			for k, v := range severities {
				labels = append(labels, model.AlertLabel{
					Name:  k,
					Value: fmt.Sprintf("%v", v),
				})
			}
			status := "firing"
			if contextRoot.IsExpired() {
				status = "resolved"
			}
			alert := model.Alert{
				Labels:      labels,
				Annotations: []model.AlertAnnotation{},
				Status:      status,
				StartsAt:    contextRoot.UpdatedAt,
				EndsAt:      contextRoot.UpdatedAt.Add(contextRoot.TTL),
			}
			hash := sha256.Sum256([]byte(contextRoot.ImageId + "/" + contextRoot.Key))
			alert.Fingerprint = "sha256:" + hex.EncodeToString(hash[:])
			var value model.Alert
			var query = model.Alert{
				Fingerprint: alert.Fingerprint,
			}
			if err := databaseContext.DB.Find(&value, &query).Error; err != nil {
				databaseContext.Logger.Error().Err(err).Msg("Failed to retrieve Alert")
				continue
			}
			if value.Fingerprint == "" {
				databaseContext.Logger.Info().Str("fingerprint", alert.Fingerprint).Str("imageid", item.ImageId).Str("imagespec", item.ImageSpec).Str("status", alert.Status).Msg("Creating new alert")
				databaseContext.DB.Create(&alert)
			} else {
				databaseContext.Logger.Info().Str("fingerprint", alert.Fingerprint).Str("imageid", item.ImageId).Str("imagespec", item.ImageSpec).Str("status", alert.Status).Msg("Updating existing alert")
				databaseContext.DB.Save(&alert)
			}
		}
		return item
	}
}

type Route struct {
	RouteConfig *model.RouteConfig
	Alerts      []model.Alert
	AlertGroups []AlertGroup
	Logger      *zerolog.Logger
	// B-Tree structure
	FirstChild  *Route
	NextSibling *Route
}

func NewRoute(routeConfig *model.RouteConfig, alerts []model.Alert) *Route {
	r := &Route{
		RouteConfig: routeConfig,
		AlertGroups: []AlertGroup{},
		Alerts:      alerts,
		Logger:      logging.NewLogger("info", "component", "RouteConfig"),
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
	r.Alerts = r.GetMatchedAlerts()
	// Handle child
	if len(routeConfig.ChildRoutes) > 0 {
		r.FirstChild = NewRoute(r.GetRouteConfigForChild(routeConfig.ChildRoutes[0]), alerts)
	}
	// Handle siblings of child
	if len(routeConfig.ChildRoutes) > 1 {
		current := r.FirstChild
		for i := 1; i < len(routeConfig.ChildRoutes); i++ {
			next := NewRoute(r.GetRouteConfigForChild(routeConfig.ChildRoutes[i]), r.GetSiblingAlerts())
			current.NextSibling = next
			current = next
		}
	}
	return r
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

func (r *Route) getMatchedAlerts(invert bool) []model.Alert {
	var matchedAlerts []model.Alert
	var unmatchedAlerts []model.Alert
	if len(r.RouteConfig.Matchers) == 0 {
		return r.Alerts
	}
	for _, alert := range r.Alerts {
		matched := true
		for _, matcherString := range r.RouteConfig.Matchers {
			if matcher, err := NewMatcher(matcherString); err != nil {
				r.Logger.Error().Err(err).Msg("Failed to create matcher")
				continue
			} else {
				if !matcher.Matches(alert) {
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
	return matchedAlerts
}

func (r *Route) GetMatchedAlerts() []model.Alert {
	return r.getMatchedAlerts(false)
}

func (r *Route) GetUnmatchedAlerts() []model.Alert {
	return r.getMatchedAlerts(true)
}

func (r *Route) GetSiblingAlerts() []model.Alert {
	if r.RouteConfig.Continue {
		return r.Alerts
	}
	return r.GetUnmatchedAlerts()
}

// A grouped alert is dependend on a route.
type AlertGroup struct {
	RouteConfig        *model.RouteConfig
	Alerts             []model.Alert
	ChildGroupedAlerts []AlertGroup
}

func NewAlertGroup(routeConfig *model.RouteConfig, alerts []model.Alert) *AlertGroup {
	return &AlertGroup{
		RouteConfig:        routeConfig,
		Alerts:             alerts,
		ChildGroupedAlerts: []AlertGroup{},
	}
}

type Matcher struct {
	MatcherString string
	Label         string
	Operator      string
	Value         string
}

func NewMatcher(matcherString string) (*Matcher, error) {
	regex := regexp.MustCompile(`^(?P<label>[^=<>!]+?) *(?P<operator>[~<>!=ß]+)[" ]*(?P<value>.+?)[" ]*$`)
	matches := regex.FindStringSubmatch(matcherString)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid matcher string: %s", matcherString)
	}
	label := matches[1]
	operator := matches[2]
	value := matches[3]
	if operator == "=~" || operator == "!~" {
		_, err := regexp.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %s", value)
		}
	}
	return &Matcher{
		MatcherString: matcherString,
		Label:         label,
		Operator:      operator,
		Value:         value,
	}, nil
}

func (m *Matcher) Matches(alert model.Alert) bool {
	prometheusAlert := GetPrometheusAlert(&alert)
	label := prometheusAlert.Labels[m.Label]
	switch m.Operator {
	case "=":
		return label == m.Value
	case "!=":
		return label != m.Value
	case "<":
		return label < m.Value
	case "<=":
		return label <= m.Value
	case ">":
		return label > m.Value
	case ">=":
		return label >= m.Value
	case "=~":
		matched, _ := regexp.MatchString(m.Value, label)
		return matched
	case "!~":
		matched, _ := regexp.MatchString(m.Value, label)
		return !matched
	default:
		return false
	}
}
