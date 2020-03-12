package line

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/statsd_exporter/pkg/event"
	"github.com/prometheus/statsd_exporter/pkg/mapper"
)

func buildEvent(statType, metric string, value float64, relative bool, labels map[string]string) (event.Event, error) {
	switch statType {
	case "c":
		return &event.CounterEvent{
			CMetricName: metric,
			CValue:      float64(value),
			CLabels:     labels,
		}, nil
	case "g":
		return &event.GaugeEvent{
			GMetricName: metric,
			GValue:      float64(value),
			GRelative:   relative,
			GLabels:     labels,
		}, nil
	case "ms", "h", "d":
		return &event.TimerEvent{
			TMetricName: metric,
			TValue:      float64(value),
			TLabels:     labels,
		}, nil
	case "s":
		return nil, fmt.Errorf("no support for StatsD sets")
	default:
		return nil, fmt.Errorf("bad stat type %s", statType)
	}
}

func parseTag(component, tag string, separator rune, labels map[string]string, tagErrors prometheus.Counter, logger log.Logger) {
	// Entirely empty tag is an error
	if len(tag) == 0 {
		tagErrors.Inc()
		level.Debug(logger).Log("msg", "Empty name tag", "component", component)
		return
	}

	for i, c := range tag {
		if c == separator {
			k := tag[:i]
			v := tag[i+1:]

			if len(k) == 0 || len(v) == 0 {
				// Empty key or value is an error
				tagErrors.Inc()
				level.Debug(logger).Log("msg", "Malformed name tag", "k", k, "v", v, "component", component)
			} else {
				labels[mapper.EscapeMetricName(k)] = v
			}
			return
		}
	}

	// Missing separator (no value) is an error
	tagErrors.Inc()
	level.Debug(logger).Log("msg", "Malformed name tag", "tag", tag, "component", component)
}

func parseNameTags(component string, labels map[string]string, tagErrors prometheus.Counter, logger log.Logger) {
	lastTagEndIndex := 0
	for i, c := range component {
		if c == ',' {
			tag := component[lastTagEndIndex:i]
			lastTagEndIndex = i + 1
			parseTag(component, tag, '=', labels, tagErrors, logger)
		}
	}

	// If we're not off the end of the string, add the last tag
	if lastTagEndIndex < len(component) {
		tag := component[lastTagEndIndex:]
		parseTag(component, tag, '=', labels, tagErrors, logger)
	}
}

func trimLeftHash(s string) string {
	if s != "" && s[0] == '#' {
		return s[1:]
	}
	return s
}

func ParseDogStatsDTags(component string, labels map[string]string, tagErrors prometheus.Counter, logger log.Logger) {
	lastTagEndIndex := 0
	for i, c := range component {
		if c == ',' {
			tag := component[lastTagEndIndex:i]
			lastTagEndIndex = i + 1
			parseTag(component, trimLeftHash(tag), ':', labels, tagErrors, logger)
		}
	}

	// If we're not off the end of the string, add the last tag
	if lastTagEndIndex < len(component) {
		tag := component[lastTagEndIndex:]
		parseTag(component, trimLeftHash(tag), ':', labels, tagErrors, logger)
	}
}

func parseNameAndTags(name string, labels map[string]string, tagErrors prometheus.Counter, logger log.Logger) string {
	for i, c := range name {
		// `#` delimits start of tags by Librato
		// https://www.librato.com/docs/kb/collect/collection_agents/stastd/#stat-level-tags
		// `,` delimits start of tags by InfluxDB
		// https://www.influxdata.com/blog/getting-started-with-sending-statsd-metrics-to-telegraf-influxdb/#introducing-influx-statsd
		if c == '#' || c == ',' {
			parseNameTags(name[i+1:], labels, tagErrors, logger)
			return name[:i]
		}
	}
	return name
}

func LineToEvents(line string, sampleErrors prometheus.CounterVec, samplesReceived prometheus.Counter, tagErrors prometheus.Counter, tagsReceived prometheus.Counter, logger log.Logger) event.Events {
	events := event.Events{}
	if line == "" {
		return events
	}

	elements := strings.SplitN(line, ":", 2)
	if len(elements) < 2 || len(elements[0]) == 0 || !utf8.ValidString(line) {
		sampleErrors.WithLabelValues("malformed_line").Inc()
		level.Debug(logger).Log("msg", "Bad line from StatsD", "line", line)
		return events
	}

	labels := map[string]string{}
	metric := parseNameAndTags(elements[0], labels, tagErrors, logger)

	var samples []string
	if strings.Contains(elements[1], "|#") {
		// using DogStatsD tags

		// don't allow mixed tagging styles
		if len(labels) > 0 {
			sampleErrors.WithLabelValues("mixed_tagging_styles").Inc()
			level.Debug(logger).Log("msg", "Bad line (multiple tagging styles) from StatsD", "line", line)
			return events
		}

		// disable multi-metrics
		samples = elements[1:]
	} else {
		samples = strings.Split(elements[1], ":")
	}

samples:
	for _, sample := range samples {
		samplesReceived.Inc()
		components := strings.Split(sample, "|")
		samplingFactor := 1.0
		if len(components) < 2 || len(components) > 4 {
			sampleErrors.WithLabelValues("malformed_component").Inc()
			level.Debug(logger).Log("msg", "Bad component", "line", line)
			continue
		}
		valueStr, statType := components[0], components[1]

		var relative = false
		if strings.Index(valueStr, "+") == 0 || strings.Index(valueStr, "-") == 0 {
			relative = true
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			level.Debug(logger).Log("msg", "Bad value", "value", valueStr, "line", line)
			sampleErrors.WithLabelValues("malformed_value").Inc()
			continue
		}

		multiplyEvents := 1
		if len(components) >= 3 {
			for _, component := range components[2:] {
				if len(component) == 0 {
					level.Debug(logger).Log("msg", "Empty component", "line", line)
					sampleErrors.WithLabelValues("malformed_component").Inc()
					continue samples
				}
			}

			for _, component := range components[2:] {
				switch component[0] {
				case '@':

					samplingFactor, err = strconv.ParseFloat(component[1:], 64)
					if err != nil {
						level.Debug(logger).Log("msg", "Invalid sampling factor", "component", component[1:], "line", line)
						sampleErrors.WithLabelValues("invalid_sample_factor").Inc()
					}
					if samplingFactor == 0 {
						samplingFactor = 1
					}

					if statType == "g" {
						continue
					} else if statType == "c" {
						value /= samplingFactor
					} else if statType == "ms" || statType == "h" || statType == "d" {
						multiplyEvents = int(1 / samplingFactor)
					}
				case '#':
					ParseDogStatsDTags(component[1:], labels, tagErrors, logger)
				default:
					level.Debug(logger).Log("msg", "Invalid sampling factor or tag section", "component", components[2], "line", line)
					sampleErrors.WithLabelValues("invalid_sample_factor").Inc()
					continue
				}
			}
		}

		if len(labels) > 0 {
			tagsReceived.Inc()
		}

		for i := 0; i < multiplyEvents; i++ {
			event, err := buildEvent(statType, metric, value, relative, labels)
			if err != nil {
				level.Debug(logger).Log("msg", "Error building event", "line", line, "error", err)
				sampleErrors.WithLabelValues("illegal_event").Inc()
				continue
			}
			events = append(events, event)
		}
	}
	return events
}
