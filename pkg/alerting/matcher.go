package alerting

import (
	"fmt"
	"regexp"

	"github.com/metraction/pharos/pkg/model"
)

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
