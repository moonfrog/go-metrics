package metrics

import (
	"strings"
)

const (
	TAG_DELIMITER        = "|"
	TAG_METRIC_DELIMITER = "TAG"
)

type TagBoard struct {
	Ns  string
	Grp string
	Tgt string
	Act string
}

func (this TagBoard) String() string {
	tags := []string{this.Grp, this.Tgt, this.Act}
	tagStr := this.Ns
	for _, tag := range tags {
		if tag != "" {
			tagStr = tagStr + TAG_DELIMITER + tag
		}
	}
	return tagStr
}

func tagMap(tbString string) map[string]string {
	tags := strings.Split(tbString, TAG_DELIMITER)
	res := make(map[string]string)
	for i, tag := range tags {
		switch i {
		case 0:
			res["ns"] = tag
		case 1:
			res["grp"] = tag
		case 2:
			res["tgt"] = tag
		case 3:
			res["act"] = tag
		}
	}
	return res
}

func TaggedMetricName(name string, tb TagBoard) string {
	return tb.String() + TAG_METRIC_DELIMITER + name
}

func IsTagged(name string) bool {
	return strings.Contains(name, TAG_METRIC_DELIMITER)
}

func ParseTaggedMetric(name string) (string, map[string]string) {
	fields := strings.Split(name, TAG_METRIC_DELIMITER)
	return fields[1], tagMap(fields[0])
}