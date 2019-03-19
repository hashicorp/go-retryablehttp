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
			[]string{"total"},
		),
		doCallFailureCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doCallFailureCount,
				Help: "Number of http Client.Do failed calls",
			},
			[]string{""},
		),
		doCallSuccessCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doCallSuccessCount,
				Help: "Number of http Client.Do calls that succeeded",
			},
			[]string{"total"},
		),
		doRetryCallCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doRetryCallCount,
				Help: "Number of http Client.Do retry calls",
			},
			[]string{"total"},
		),
		doRetryCallFailureCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: doRetryCallFailureCount,
				Help: "Number of http Client.Do failed  retry calls",
			},
			[]string{"total"},
		),
		doDuration: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       doDuration,
				Help:       "Durations per http request made in a summary vector",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
			},
			[]string{"request_duration"},
		),
		retryDuration: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Name:       retryDuration,
				Help:       "Durations per http request retry in a summary vector",
				Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.95: 0.005, 0.99: 0.001},
			},
			[]string{"request_duration"},
		),
	}

	if err := registerMetrics(prometheusMetrics); err != nil {
		return nil, err
	}

	var doCalls = prometheusMetrics[doCallCount].(*prometheus.CounterVec)
	var doCallSuccess = prometheusMetrics[doCallSuccessCount].(*prometheus.CounterVec)
	var doCallFailures = prometheusMetrics[doCallFailureCount].(*prometheus.CounterVec)

	var doRetries = prometheusMetrics[doRetryCallCount].(*prometheus.CounterVec)
	var doRetriesFailures = prometheusMetrics[doRetryCallFailureCount].(*prometheus.CounterVec)

	var doDurations = prometheusMetrics[doDuration].(*prometheus.SummaryVec)
	var doRetryDurations = prometheusMetrics[retryDuration].(*prometheus.SummaryVec)

	var metrics = &retryHttpMetrics{
		// do counters
		doTotal:   doCalls.WithLabelValues("http.do.total"),
		doFailure: doCallFailures.WithLabelValues("http.do.failed"),
		doSuccess: doCallSuccess.WithLabelValues("http.do.succeeded"),

		// retry counters
		doRetries:        doRetries.WithLabelValues("http.do.retires"),
		doRetriesFailure: doRetriesFailures.WithLabelValues("http.do.retries.failed"),

		// durations
		doDuration:      doDurations.WithLabelValues("http.do.duration"),
		doRetryDuration: doRetryDurations.WithLabelValues("http.do.retry.duration"),
	}
	return metrics, nil
}

type retryHttpMetrics struct {
	doTotal          prometheus.Counter
	doSuccess        prometheus.Counter
	doFailure        prometheus.Counter
	doRetries        prometheus.Counter
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
