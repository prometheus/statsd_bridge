// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"reflect"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/statsd_exporter/pkg/event"
	"github.com/prometheus/statsd_exporter/pkg/listener"
)

func TestHandlePacket(t *testing.T) {
	scenarios := []struct {
		name string
		in   string
		out  event.Events
	}{
		{
			name: "empty",
		}, {
			name: "simple counter",
			in:   "foo:2|c",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      2,
					CLabels:     map[string]string{},
				},
			},
		}, {
			name: "simple gauge",
			in:   "foo:3|g",
			out: event.Events{
				&event.GaugeEvent{
					GMetricName: "foo",
					GValue:      3,
					GLabels:     map[string]string{},
				},
			},
		}, {
			name: "gauge with sampling",
			in:   "foo:3|g|@0.2",
			out: event.Events{
				&event.GaugeEvent{
					GMetricName: "foo",
					GValue:      3,
					GLabels:     map[string]string{},
				},
			},
		}, {
			name: "gauge decrement",
			in:   "foo:-10|g",
			out: event.Events{
				&event.GaugeEvent{
					GMetricName: "foo",
					GValue:      -10,
					GRelative:   true,
					GLabels:     map[string]string{},
				},
			},
		}, {
			name: "simple timer",
			in:   "foo:200|ms",
			out: event.Events{
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      200,
					TLabels:     map[string]string{},
				},
			},
		}, {
			name: "simple histogram",
			in:   "foo:200|h",
			out: event.Events{
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      200,
					TLabels:     map[string]string{},
				},
			},
		}, {
			name: "simple distribution",
			in:   "foo:200|d",
			out: event.Events{
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      200,
					TLabels:     map[string]string{},
				},
			},
		}, {
			name: "distribution with sampling",
			in:   "foo:0.01|d|@0.2|#tag1:bar,#tag2:baz",
			out: event.Events{
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "librato tag extension",
			in:   "foo#tag1=bar,tag2=baz:100|c",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "librato tag extension with tag keys unsupported by prometheus",
			in:   "foo#09digits=0,tag.with.dots=1:100|c",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"_09digits": "0", "tag_with_dots": "1"},
				},
			},
		}, {
			name: "influxdb tag extension",
			in:   "foo,tag1=bar,tag2=baz:100|c",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "influxdb tag extension with tag keys unsupported by prometheus",
			in:   "foo,09digits=0,tag.with.dots=1:100|c",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"_09digits": "0", "tag_with_dots": "1"},
				},
			},
		}, {
			name: "datadog tag extension",
			in:   "foo:100|c|#tag1:bar,tag2:baz",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "datadog tag extension with # in all keys (as sent by datadog php client)",
			in:   "foo:100|c|#tag1:bar,#tag2:baz",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "datadog tag extension with tag keys unsupported by prometheus",
			in:   "foo:100|c|#09digits:0,tag.with.dots:1",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"_09digits": "0", "tag_with_dots": "1"},
				},
			},
		}, {
			name: "datadog tag extension with valueless tags: ignored",
			in:   "foo:100|c|#tag_without_a_value",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{},
				},
			},
		}, {
			name: "datadog tag extension with valueless tags (edge case)",
			in:   "foo:100|c|#tag_without_a_value,tag:value",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"tag": "value"},
				},
			},
		}, {
			name: "datadog tag extension with empty tags (edge case)",
			in:   "foo:100|c|#tag:value,,",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      100,
					CLabels:     map[string]string{"tag": "value"},
				},
			},
		}, {
			name: "datadog tag extension with sampling",
			in:   "foo:100|c|@0.1|#tag1:bar,#tag2:baz",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      1000,
					CLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "librato/dogstatsd mixed tag styles without sampling",
			in:   "foo#tag1=foo,tag3=bing:100|c|#tag1:bar,#tag2:baz",
			out:  event.Events{},
		}, {
			name: "influxdb/dogstatsd mixed tag styles without sampling",
			in:   "foo,tag1=foo,tag3=bing:100|c|#tag1:bar,#tag2:baz",
			out:  event.Events{},
		}, {
			name: "mixed tag styles with sampling",
			in:   "foo#tag1=foo,tag3=bing:100|c|@0.1|#tag1:bar,#tag2:baz",
			out:  event.Events{},
		}, {
			name: "histogram with sampling",
			in:   "foo:0.01|h|@0.2|#tag1:bar,#tag2:baz",
			out: event.Events{
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      0.01,
					TLabels:     map[string]string{"tag1": "bar", "tag2": "baz"},
				},
			},
		}, {
			name: "datadog tag extension with multiple colons",
			in:   "foo:100|c|@0.1|#tag1:foo:bar",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      1000,
					CLabels:     map[string]string{"tag1": "foo:bar"},
				},
			},
		}, {
			name: "datadog tag extension with invalid utf8 tag values",
			in:   "foo:100|c|@0.1|#tag:\xc3\x28invalid",
		}, {
			name: "datadog tag extension with both valid and invalid utf8 tag values",
			in:   "foo:100|c|@0.1|#tag1:valid,tag2:\xc3\x28invalid",
		}, {
			name: "multiple metrics with invalid datadog utf8 tag values",
			in:   "foo:200|c|#tag:value\nfoo:300|c|#tag:\xc3\x28invalid",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      200,
					CLabels:     map[string]string{"tag": "value"},
				},
			},
		}, {
			name: "combined multiline metrics",
			in:   "foo:200|ms:300|ms:5|c|@0.1:6|g\nbar:1|c:5|ms",
			out: event.Events{
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      200,
					TLabels:     map[string]string{},
				},
				&event.TimerEvent{
					TMetricName: "foo",
					TValue:      300,
					TLabels:     map[string]string{},
				},
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      50,
					CLabels:     map[string]string{},
				},
				&event.GaugeEvent{
					GMetricName: "foo",
					GValue:      6,
					GLabels:     map[string]string{},
				},
				&event.CounterEvent{
					CMetricName: "bar",
					CValue:      1,
					CLabels:     map[string]string{},
				},
				&event.TimerEvent{
					TMetricName: "bar",
					TValue:      5,
					TLabels:     map[string]string{},
				},
			},
		}, {
			name: "timings with sampling factor",
			in:   "foo.timing:0.5|ms|@0.1",
			out: event.Events{
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
				&event.TimerEvent{TMetricName: "foo.timing", TValue: 0.5, TLabels: map[string]string{}},
			},
		}, {
			name: "bad line",
			in:   "foo",
		}, {
			name: "bad component",
			in:   "foo:1",
		}, {
			name: "bad value",
			in:   "foo:1o|c",
		}, {
			name: "illegal sampling factor",
			in:   "foo:1|c|@bar",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      1,
					CLabels:     map[string]string{},
				},
			},
		}, {
			name: "zero sampling factor",
			in:   "foo:2|c|@0",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "foo",
					CValue:      2,
					CLabels:     map[string]string{},
				},
			},
		}, {
			name: "illegal stat type",
			in:   "foo:2|t",
		},
		{
			name: "empty metric name",
			in:   ":100|ms",
		},
		{
			name: "empty component",
			in:   "foo:1|c|",
		},
		{
			name: "invalid utf8",
			in:   "invalid\xc3\x28utf8:1|c",
		},
		{
			name: "some invalid utf8",
			in:   "valid_utf8:1|c\ninvalid\xc3\x28utf8:1|c",
			out: event.Events{
				&event.CounterEvent{
					CMetricName: "valid_utf8",
					CValue:      1,
					CLabels:     map[string]string{},
				},
			},
		},
	}

	for k, l := range []statsDPacketHandler{&listener.StatsDUDPListener{nil, nil, log.NewNopLogger()}, &mockStatsDTCPListener{listener.StatsDTCPListener{nil, nil, log.NewNopLogger()}, log.NewNopLogger()}} {
		events := make(chan event.Events, 32)
		l.SetEventHandler(&event.UnbufferedEventHandler{C: events})
		for i, scenario := range scenarios {
			l.HandlePacket([]byte(scenario.in), udpPackets, linesReceived, eventsFlushed, *sampleErrors, samplesReceived, tagErrors, tagsReceived)

			le := len(events)
			// Flatten actual events.
			actual := event.Events{}
			for i := 0; i < le; i++ {
				actual = append(actual, <-events...)
			}

			if len(actual) != len(scenario.out) {
				t.Fatalf("%d.%d. Expected %d events, got %d in scenario '%s'", k, i, len(scenario.out), len(actual), scenario.name)
			}

			for j, expected := range scenario.out {
				if !reflect.DeepEqual(&expected, &actual[j]) {
					t.Fatalf("%d.%d.%d. Expected %#v, got %#v in scenario '%s'", k, i, j, expected, actual[j], scenario.name)
				}
			}
		}
	}
}
