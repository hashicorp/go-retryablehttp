package retryablehttp

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	doCallCount        = "http_client_do_count"
	doCallFailureCount = "http_client_do_failure_count"
	doCallSuccessCount = "http_client_do_success_count"

	doRetryCallCount        = "http_client_retry_do_count"
	doRetryCallFailureCount = "http_client_retry_do_failure_count"
	doRetryCallSuccessCount = "http_client_retry_do_success_count"

	doDuration    = "http_client_task_duration"
	retryDuration = "http_client_retry_duration"
)

func initMetrics() (*retryHttpMetrics, error) {
	var prometheusMetrics = map[string]prometheus.Collector{
		doCallCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doCallCount,
				Help: "Number of http Client.Do calls",
			},
			[]string{"result"},
		),
		doCallFailureCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doCallFailureCount,
				Help: "Number of http Client.Do failed calls",
			},
			[]string{"result"},
		),
		doCallSuccessCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doCallSuccessCount,
				Help: "Number of http Client.Do success calls",
			},
			[]string{"result"},
		),
		doRetryCallCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doRetryCallCount,
				Help: "Number of http Client.Do calls",
			},
			[]string{"result"},
		),
		doRetryCallFailureCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doRetryCallFailureCount,
				Help: "Number of http Client.Do failed calls",
			},
			[]string{"result"},
		),
		doRetryCallSuccessCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doRetryCallSuccessCount,
				Help: "Number of http Client.Do success calls",
			},
			[]string{"result"},
		),
		doDuration: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       doDuration,
				Help:       "Durations per http request made in a summary vector",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
			},
			[]string{"request_url", "request_method", "response_status"},
		),
		retryDuration: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       retryDuration,
				Help:       "Durations per http request retry in a summary vector",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
			},
			[]string{"request_url", "request_method", "response_status"},
		),
	}

	if err := registerMetrics(prometheusMetrics); err != nil {
		return nil, err
	}

	var doCalls = prometheusMetrics[doCallCount].(*prometheus.CounterVec)
	var doCallSuccess = prometheusMetrics[doCallSuccessCount].(*prometheus.CounterVec)
	var doCallFailures = prometheusMetrics[doCallFailureCount].(*prometheus.CounterVec)

	var doRetries = prometheusMetrics[doRetryCallCount].(*prometheus.CounterVec)
	var doRetriesSuccess = prometheusMetrics[doRetryCallSuccessCount].(*prometheus.CounterVec)
	var doRetriesFailures = prometheusMetrics[doRetryCallFailureCount].(*prometheus.CounterVec)

	var doDurations = prometheusMetrics[doDuration].(*prometheus.SummaryVec)
	var doRetryDurations = prometheusMetrics[retryDuration].(*prometheus.SummaryVec)

	var metrics = &retryHttpMetrics{
		// do counters
		doTotal:   doCalls.WithLabelValues("do_total"),
		doFailure: doCallFailures.WithLabelValues("do_failed"),
		doSuccess: doCallSuccess.WithLabelValues("do_success"),

		// retry counters
		doRetries:        doRetries.WithLabelValues("retry_total"),
		doRetriesSuccess: doRetriesSuccess.WithLabelValues("retry_failed"),
		doRetriesFailure: doRetriesFailures.WithLabelValues("retry_success"),

		// durations
		doDuration:      doDurations.WithLabelValues("http_client_do_total_duration"),
		doRetryDuration: doRetryDurations.WithLabelValues("http_client_do_retry_total_duration"),
	}
	return metrics, nil
}

type retryHttpMetrics struct {
	doTotal          prometheus.Counter
	doSuccess        prometheus.Counter
	doFailure        prometheus.Counter
	doRetries        prometheus.Counter
	doRetriesSuccess prometheus.Counter
	doRetriesFailure prometheus.Counter
	doDuration       prometheus.Observer
	doRetryDuration  prometheus.Observer
}

func registerMetrics(m map[string]prometheus.Collector) error {
	for _, metric := range m {
		var err = prometheus.Register(metric)
		if err != nil && err != err.(prometheus.AlreadyRegisteredError) {
			return err
		}
	}
	return nil
}
