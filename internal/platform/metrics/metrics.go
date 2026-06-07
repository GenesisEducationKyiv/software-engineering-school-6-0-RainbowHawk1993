package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type ServiceMetrics struct {
	ServiceUp                  prometheus.Gauge
	HTTPRequestsTotal          *prometheus.CounterVec
	HTTPRequestDurationSeconds *prometheus.HistogramVec
	GitHubRequestsTotal        *prometheus.CounterVec
	ScannerRunsTotal           *prometheus.CounterVec
	ScannerRepositoriesTotal   prometheus.Counter
	NotificationsSentTotal     prometheus.Counter
	NotificationsFailedTotal   prometheus.Counter
}

func NewRegistry() (*prometheus.Registry, *ServiceMetrics) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	metrics := &ServiceMetrics{
		ServiceUp: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "releases_api_service_up",
			Help: "Whether the service is currently running.",
		}),
		HTTPRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "releases_api_http_requests_total",
				Help: "Total number of HTTP requests handled by the service.",
			},
			[]string{"method", "route", "status"},
		),
		HTTPRequestDurationSeconds: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "releases_api_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
		GitHubRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "releases_api_github_requests_total",
				Help: "Total number of GitHub repo and release lookups grouped by operation and outcome.",
			},
			[]string{"operation", "source", "outcome"},
		),
		ScannerRunsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "releases_api_scanner_runs_total",
				Help: "Total number of background scanner runs grouped by outcome.",
			},
			[]string{"outcome"},
		),
		ScannerRepositoriesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "releases_api_scanner_repositories_checked_total",
			Help: "Total number of repositories checked by the background scanner.",
		}),
		NotificationsSentTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "releases_api_notifications_sent_total",
			Help: "Total number of release notification emails sent successfully.",
		}),
		NotificationsFailedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "releases_api_notifications_failed_total",
			Help: "Total number of failed release notification email attempts.",
		}),
	}

	registry.MustRegister(
		metrics.ServiceUp,
		metrics.HTTPRequestsTotal,
		metrics.HTTPRequestDurationSeconds,
		metrics.GitHubRequestsTotal,
		metrics.ScannerRunsTotal,
		metrics.ScannerRepositoriesTotal,
		metrics.NotificationsSentTotal,
		metrics.NotificationsFailedTotal,
	)

	return registry, metrics
}

func (m *ServiceMetrics) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if m == nil {
		return
	}

	m.HTTPRequestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.HTTPRequestDurationSeconds.WithLabelValues(method, route).Observe(duration.Seconds())
}

func (m *ServiceMetrics) ObserveGitHubRequest(operation, source, outcome string) {
	if m == nil {
		return
	}

	m.GitHubRequestsTotal.WithLabelValues(operation, source, outcome).Inc()
}

func (m *ServiceMetrics) ObserveScannerRun(outcome string) {
	if m == nil {
		return
	}

	m.ScannerRunsTotal.WithLabelValues(outcome).Inc()
}
