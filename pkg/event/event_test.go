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

package event

import (
	"reflect"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/statsd_exporter/pkg/clock"
	"github.com/prometheus/statsd_exporter/pkg/mapper"
)

var eventsFlushed = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "statsd_exporter_event_queue_flushed_total",
		Help: "Number of times events were flushed to exporter",
	},
)

func TestEventThresholdFlush(t *testing.T) {
	c := make(chan Events, 100)
	// We're not going to flush during this test, so the duration doesn't matter.
	eq := NewEventQueue(c, 5, time.Second, eventsFlushed)
	e := make(Events, 13)
	go func() {
		eq.Queue(e)
	}()

	batch := <-c
	if len(batch) != 5 {
		t.Fatalf("Expected event batch to be 5 elements, but got %v", len(batch))
	}
	batch = <-c
	if len(batch) != 5 {
		t.Fatalf("Expected event batch to be 5 elements, but got %v", len(batch))
	}
	batch = <-c
	if len(batch) != 3 {
		t.Fatalf("Expected event batch to be 3 elements, but got %v", len(batch))
	}
}

func TestEventIntervalFlush(t *testing.T) {
	// Mock a time.NewTicker
	tickerCh := make(chan time.Time)
	clock.ClockInstance = &clock.Clock{
		TickerCh: tickerCh,
	}
	clock.ClockInstance.Instant = time.Unix(0, 0)

	c := make(chan Events, 100)
	eq := NewEventQueue(c, 1000, time.Second*1000, eventsFlushed)
	e := make(Events, 10)
	eq.Queue(e)

	if eq.Len() != 10 {
		t.Fatal("Expected 10 events to be queued, but got", eq.Len())
	}

	if len(eq.C) != 0 {
		t.Fatal("Expected 0 events in the event channel, but got", len(eq.C))
	}

	// Tick time forward to trigger a flush
	clock.ClockInstance.Instant = time.Unix(10000, 0)
	clock.ClockInstance.TickerCh <- time.Unix(10000, 0)

	events := <-eq.C
	if eq.Len() != 0 {
		t.Fatal("Expected 0 events to be queued, but got", eq.Len())
	}

	if len(events) != 10 {
		t.Fatal("Expected 10 events in the event channel, but got", len(events))
	}
}

func TestMultiValueEvent(t *testing.T) {
	tests := []struct {
		name       string
		event      MultiValueEvent
		wantValues []float64
		wantName   string
		wantType   mapper.MetricType
		wantLabels map[string]string
	}{
		{
			name: "MultiObserverEvent with single value",
			event: &MultiObserverEvent{
				OMetricName: "test_metric",
				OValues:     []float64{1.0},
				OLabels:     map[string]string{"label": "value"},
				SampleRate:  0,
			},
			wantValues: []float64{1.0},
			wantName:   "test_metric",
			wantType:   mapper.MetricTypeObserver,
			wantLabels: map[string]string{"label": "value"},
		},
		{
			name: "MultiObserverEvent with multiple values",
			event: &MultiObserverEvent{
				OMetricName: "test_metric",
				OValues:     []float64{1.0, 2.0, 3.0},
				OLabels:     map[string]string{"label": "value"},
				SampleRate:  0.5,
			},
			wantValues: []float64{1.0, 2.0, 3.0},
			wantName:   "test_metric",
			wantType:   mapper.MetricTypeObserver,
			wantLabels: map[string]string{"label": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.event.Values(); !reflect.DeepEqual(got, tt.wantValues) {
				t.Errorf("MultiValueEvent.Values() = %v, want %v", got, tt.wantValues)
			}
			if got := tt.event.MetricName(); got != tt.wantName {
				t.Errorf("MultiValueEvent.MetricName() = %v, want %v", got, tt.wantName)
			}
			if got := tt.event.MetricType(); got != tt.wantType {
				t.Errorf("MultiValueEvent.MetricType() = %v, want %v", got, tt.wantType)
			}
			if got := tt.event.Labels(); !reflect.DeepEqual(got, tt.wantLabels) {
				t.Errorf("MultiValueEvent.Labels() = %v, want %v", got, tt.wantLabels)
			}
			if got := tt.event.Value(); got != tt.wantValues[0] {
				t.Errorf("MultiValueEvent.Value() = %v, want %v", got, tt.wantValues[0])
			}
		})
	}
}

func TestMultiObserverEvent_Explode(t *testing.T) {
	tests := []struct {
		name       string
		event      *MultiObserverEvent
		wantEvents []Event
	}{
		{
			name: "single value no sampling",
			event: &MultiObserverEvent{
				OMetricName: "test_metric",
				OValues:     []float64{1.0},
				OLabels:     map[string]string{"label": "value"},
				SampleRate:  0,
			},
			wantEvents: []Event{
				&MultiObserverEvent{
					OMetricName: "test_metric",
					OValues:     []float64{1.0},
					OLabels:     map[string]string{"label": "value"},
					SampleRate:  0,
				},
			},
		},
		{
			name: "multiple values no sampling",
			event: &MultiObserverEvent{
				OMetricName: "test_metric",
				OValues:     []float64{1.0, 2.0, 3.0},
				OLabels:     map[string]string{"label": "value"},
				SampleRate:  0,
			},
			wantEvents: []Event{
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      1.0,
					OLabels:     map[string]string{"label": "value"},
				},
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      2.0,
					OLabels:     map[string]string{"label": "value"},
				},
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      3.0,
					OLabels:     map[string]string{"label": "value"},
				},
			},
		},
		{
			name: "multiple values with sampling",
			event: &MultiObserverEvent{
				OMetricName: "test_metric",
				OValues:     []float64{1.0, 2.0},
				OLabels:     map[string]string{"label": "value"},
				SampleRate:  0.5,
			},
			wantEvents: []Event{
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      1.0,
					OLabels:     map[string]string{"label": "value"},
				},
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      2.0,
					OLabels:     map[string]string{"label": "value"},
				},
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      1.0,
					OLabels:     map[string]string{"label": "value"},
				},
				&ObserverEvent{
					OMetricName: "test_metric",
					OValue:      2.0,
					OLabels:     map[string]string{"label": "value"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.Explode()
			if !reflect.DeepEqual(got, tt.wantEvents) {
				t.Errorf("MultiObserverEvent.Explode() = %v, want %v", got, tt.wantEvents)
			}
		})
	}
}

func TestEventImplementations(t *testing.T) {
	tests := []struct {
		name  string
		event interface{}
	}{
		{
			name:  "MultiObserverEvent implements MultiValueEvent",
			event: &MultiObserverEvent{},
		},
		{
			name:  "MultiObserverEvent implements ExplodableEvent",
			event: &MultiObserverEvent{},
		},
		{
			name:  "MultiObserverEvent implements Event",
			event: &MultiObserverEvent{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "MultiObserverEvent implements MultiValueEvent":
				if _, ok := tt.event.(MultiValueEvent); !ok {
					t.Error("MultiObserverEvent does not implement MultiValueEvent")
				}
			case "MultiObserverEvent implements ExplodableEvent":
				if _, ok := tt.event.(ExplodableEvent); !ok {
					t.Error("MultiObserverEvent does not implement ExplodableEvent")
				}
			case "MultiObserverEvent implements Event":
				if _, ok := tt.event.(Event); !ok {
					t.Error("MultiObserverEvent does not implement Event")
				}
			}
		})
	}
}
