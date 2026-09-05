/**
 * @file metrics.go
 * @package metrics
 * @author Dr.NP <np@herewe.tech>
 * @since 11/20/2023
 */

package metrics

import (
	"maps"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

var (
	pool = make(map[string]prometheus.Collector)
	lock sync.RWMutex
)

// Histogram buckets: fast RPC/HTTP vs slow infra operations.
// Label name constants (also satisfies goconst: these tokens repeat across
// every Vec definition).
const (
	labelServer    = "server"
	labelMethod    = "method"
	labelRoute     = "route"
	labelCode      = "code"
	labelReason    = "reason"
	labelDirection = "direction"
	labelType      = "type"
	labelClient    = "client"
	labelHost      = "host"
	labelResult    = "result"
	labelBroker    = "broker"
	labelTopic     = "topic"
	labelJob       = "job"
	labelTask      = "task"
	labelRunner    = "runner"
	labelBackend   = "backend"
	labelOp        = "op"
	labelInfra     = "infra"
	labelComponent = "component"
	labelService   = "service"
)

var (
	// DefaultBuckets covers unary RPC / HTTP handlers.
	DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
	// InfraBuckets covers SQL / Mongo / Elastic / S3 slow paths.
	InfraBuckets = []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 15, 30}
)

var (
	// Server Metrics (RED + connections).

	// ServerRequestsTotal counts inbound requests/messages.
	ServerRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_server_requests_total",
			Help: "Number of inbound server requests/messages by server, method/route and code.",
		},
		[]string{labelServer, labelMethod, labelRoute, labelCode},
	)
	// ServerRequestDuration observes handler latency.
	ServerRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_server_request_duration_seconds",
			Help:    "Inbound server request latency by server and method/route.",
			Buckets: DefaultBuckets,
		},
		[]string{labelServer, labelMethod, labelRoute},
	)
	// ServerRejectedTotal counts pre-handler rejections (CORS, body-limit,
	// session caps, rate limits, origin checks).
	ServerRejectedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_server_rejected_total",
			Help: "Number of rejected requests/messages by server and reason.",
		},
		[]string{labelServer, labelReason},
	)
	// ServerConnections tracks live sessions/connections.
	ServerConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_server_connections",
			Help: "Current live server connections/sessions by server.",
		},
		[]string{labelServer},
	)
	// ServerPanicsTotal counts recovered panics in handlers.
	ServerPanicsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_server_panics_total",
			Help: "Number of recovered handler panics by server and method.",
		},
		[]string{labelServer, labelMethod},
	)
	// ServerIOBytesTotal counts payload bytes per direction.
	ServerIOBytesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_server_io_bytes_total",
			Help: "Inbound/outbound payload bytes by server and direction.",
		},
		[]string{labelServer, labelDirection},
	)

	// Client Metrics.

	// ClientRequestsTotal counts outbound calls.
	ClientRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_client_requests_total",
			Help: "Number of outbound client calls by client, method/host and code.",
		},
		[]string{labelClient, labelMethod, labelHost, labelCode},
	)
	// ClientRequestDuration observes outbound call latency.
	ClientRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_client_request_duration_seconds",
			Help:    "Outbound client call latency by client and method/host.",
			Buckets: DefaultBuckets,
		},
		[]string{labelClient, labelMethod, labelHost},
	)
	// ClientErrorsTotal counts transport-level failures.
	ClientErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_client_errors_total",
			Help: "Number of failed outbound client calls by client, method and reason.",
		},
		[]string{labelClient, labelMethod, labelReason},
	)
	// ClientStreamOpenTotal counts streaming open attempts.
	ClientStreamOpenTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_client_stream_open_total",
			Help: "Number of outbound streaming open attempts by client, method and result.",
		},
		[]string{labelClient, labelMethod, labelResult},
	)

	// Broker Metrics.

	// BrokerPublishTotal counts publish attempts.
	BrokerPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_broker_publish_total",
			Help: "Number of broker publish attempts by broker, topic and result.",
		},
		[]string{labelBroker, labelTopic, labelResult},
	)
	// BrokerPublishDuration observes publish latency.
	BrokerPublishDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_broker_publish_duration_seconds",
			Help:    "Broker publish latency by broker and topic.",
			Buckets: DefaultBuckets,
		},
		[]string{labelBroker, labelTopic},
	)
	// BrokerHandlerTotal counts handler outcomes (ok/error/panic/acked/nacked/requeued/dup).
	BrokerHandlerTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_broker_handler_total",
			Help: "Number of broker handler outcomes by broker, topic and result.",
		},
		[]string{labelBroker, labelTopic, labelResult},
	)
	// BrokerHandlerDuration observes handler latency.
	BrokerHandlerDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_broker_handler_duration_seconds",
			Help:    "Broker handler latency by broker and topic.",
			Buckets: DefaultBuckets,
		},
		[]string{labelBroker, labelTopic},
	)
	// BrokerSubscribeTotal counts subscribe/unsubscribe attempts.
	BrokerSubscribeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_broker_subscribe_total",
			Help: "Number of broker subscribe attempts by broker, topic and result.",
		},
		[]string{labelBroker, labelTopic, labelResult},
	)
	// BrokerConnected reports transport connectivity (1/0).
	BrokerConnected = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_broker_connected",
			Help: "Broker transport connectivity by broker (1 connected, 0 down).",
		},
		[]string{labelBroker},
	)

	// Job Metrics.

	// JobRegisteredTotal counts task registrations.
	JobRegisteredTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_job_registered_total",
			Help: "Number of registered job tasks by job type and result.",
		},
		[]string{labelJob, labelResult},
	)
	// JobRunsTotal counts run outcomes (ok/error/timeout/panic).
	JobRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_job_runs_total",
			Help: "Number of job run outcomes by job type, task and result.",
		},
		[]string{labelJob, labelTask, labelResult},
	)
	// JobRunDuration observes run latency.
	JobRunDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_job_run_duration_seconds",
			Help:    "Job run latency by job type.",
			Buckets: DefaultBuckets,
		},
		[]string{labelJob},
	)
	// JobTicksTotal counts scheduler ticks.
	JobTicksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_job_ticks_total",
			Help: "Number of scheduler ticks by job type and result.",
		},
		[]string{labelJob, labelResult},
	)
	// JobRunning reports scheduler state (1/0).
	JobRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_job_running",
			Help: "Job scheduler running state by job type (1 running, 0 stopped).",
		},
		[]string{labelJob},
	)

	// Runner Metrics.

	// RunnerSubmitTotal counts blocking submit outcomes.
	RunnerSubmitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_runner_submit_total",
			Help: "Number of runner blocking-submit outcomes by result.",
		},
		[]string{labelRunner, labelResult},
	)
	// RunnerTrySubmitTotal counts TryTask outcomes (incl. pool-full back-pressure).
	RunnerTrySubmitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_runner_try_submit_total",
			Help: "Number of runner TryTask outcomes by result.",
		},
		[]string{labelRunner, labelResult},
	)
	// RunnerTaskRunsTotal counts worker execution outcomes.
	RunnerTaskRunsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_runner_task_runs_total",
			Help: "Number of runner task execution outcomes by result.",
		},
		[]string{labelRunner, labelResult},
	)
	// RunnerTaskDuration observes task execution latency.
	RunnerTaskDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_runner_task_duration_seconds",
			Help:    "Runner task execution latency.",
			Buckets: DefaultBuckets,
		},
		[]string{labelRunner},
	)
	// RunnerQueueDepth reports queued (not yet picked-up) tasks.
	RunnerQueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_runner_queue_depth",
			Help: "Current runner queued task depth.",
		},
		[]string{labelRunner},
	)
	// RunnerInflight reports tasks currently executing in handlers.
	RunnerInflight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_runner_inflight",
			Help: "Current runner in-flight task count.",
		},
		[]string{labelRunner},
	)
	// RunnerWorkers reports configured worker count while started.
	RunnerWorkers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_runner_workers",
			Help: "Current runner worker count.",
		},
		[]string{labelRunner},
	)

	// Registry Metrics.

	// RegistryOpsTotal counts registry operations by backend/op/result.
	RegistryOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_registry_ops_total",
			Help: "Number of registry operations by backend, op and result.",
		},
		[]string{labelBackend, labelOp, labelResult},
	)
	// RegistryOpDuration observes registry operation latency.
	RegistryOpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_registry_op_duration_seconds",
			Help:    "Registry operation latency by backend and op.",
			Buckets: DefaultBuckets,
		},
		[]string{labelBackend, labelOp},
	)
	// RegistryInstances reports last Load() instance count.
	RegistryInstances = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_registry_instances",
			Help: "Last observed registry instance count by backend.",
		},
		[]string{labelBackend},
	)
	// RegistryWatchEventsTotal counts watcher events.
	RegistryWatchEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_registry_watch_events_total",
			Help: "Number of registry watcher events by backend and type.",
		},
		[]string{labelBackend, labelType},
	)

	// Infra Metrics (operation instrumentation; no polling collector yet).

	// InfraOpsTotal counts infra operations by component/op/result.
	InfraOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_infra_ops_total",
			Help: "Number of infra operations by component, op and result.",
		},
		[]string{labelInfra, labelOp, labelResult},
	)
	// InfraOpDuration observes infra operation latency.
	InfraOpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_infra_op_duration_seconds",
			Help:    "Infra operation latency by component and op.",
			Buckets: InfraBuckets,
		},
		[]string{labelInfra, labelOp},
	)
	// InfraUp reports singleton presence/connected state (1/0).
	InfraUp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sicky_infra_up",
			Help: "Infra singleton up state by component (1 up, 0 down).",
		},
		[]string{labelInfra},
	)
	// InfraInitTotal counts Init attempts.
	InfraInitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_infra_init_total",
			Help: "Number of infra init attempts by component and result.",
		},
		[]string{labelInfra, labelResult},
	)

	// Manager / orchestrator Metrics.

	// ManagerHealthCheckDuration observes per-component health probe latency.
	ManagerHealthCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sicky_manager_health_check_duration_seconds",
			Help:    "Manager health probe latency by component.",
			Buckets: DefaultBuckets,
		},
		[]string{labelComponent},
	)
	// ConfigReloadsTotal counts SIGHUP reload handler outcomes.
	ConfigReloadsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_config_reloads_total",
			Help: "Number of config reload handler outcomes by result.",
		},
		[]string{labelResult},
	)
	// ServiceStartsTotal counts orchestrated service start outcomes.
	ServiceStartsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sicky_service_starts_total",
			Help: "Number of service start outcomes by service and result.",
		},
		[]string{labelService, labelResult},
	)
)

// ResultOf maps an error to a label value. Timeout/panic callers pass
// explicit result strings instead of using this helper.
func ResultOf(err error) string {
	if err == nil {
		return "ok"
	}

	return "error"
}

// ObserveInfraOp records one infra operation (counter + histogram).
// op is a low-cardinality verb (e.g. "query", "get", "publish", "put");
// never pass raw SQL, URLs or topic payloads here.
func ObserveInfraOp(infra, op string, start time.Time, err error) {
	result := ResultOf(err)
	InfraOpsTotal.WithLabelValues(infra, op, result).Inc()
	InfraOpDuration.WithLabelValues(infra, op).Observe(time.Since(start).Seconds())
}

// SetInfraUp publishes singleton presence/connected state.
func SetInfraUp(infra string, up bool) {
	if up {
		InfraUp.WithLabelValues(infra).Set(1)
	} else {
		InfraUp.WithLabelValues(infra).Set(0)
	}
}

// CountInfraInit records an Init attempt outcome.
func CountInfraInit(infra string, err error) {
	InfraInitTotal.WithLabelValues(infra, ResultOf(err)).Inc()
	SetInfraUp(infra, err == nil)
}

// ObserveServerRequest records one inbound request/message.
func ObserveServerRequest(server, method, route, code string, d time.Duration) {
	ServerRequestsTotal.WithLabelValues(server, method, route, code).Inc()
	ServerRequestDuration.WithLabelValues(server, method, route).Observe(d.Seconds())
}

// ObserveClientRequest records one outbound call.
func ObserveClientRequest(client, method, host, code string, d time.Duration) {
	ClientRequestsTotal.WithLabelValues(client, method, host, code).Inc()
	ClientRequestDuration.WithLabelValues(client, method, host).Observe(d.Seconds())
}

// ObserveBrokerPublish records one publish attempt.
func ObserveBrokerPublish(broker, topic string, start time.Time, err error) {
	result := ResultOf(err)
	BrokerPublishTotal.WithLabelValues(broker, topic, result).Inc()
	BrokerPublishDuration.WithLabelValues(broker, topic).Observe(time.Since(start).Seconds())
}

// ObserveBrokerHandler records one handler outcome.
func ObserveBrokerHandler(broker, topic, result string, d time.Duration) {
	BrokerHandlerTotal.WithLabelValues(broker, topic, result).Inc()
	BrokerHandlerDuration.WithLabelValues(broker, topic).Observe(d.Seconds())
}

// ObserveJobRun records one job run outcome.
func ObserveJobRun(job, task, result string, d time.Duration) {
	JobRunsTotal.WithLabelValues(job, task, result).Inc()
	JobRunDuration.WithLabelValues(job).Observe(d.Seconds())
}

// ObserveRegistryOp records one registry operation.
func ObserveRegistryOp(backend, op string, start time.Time, err error) {
	result := ResultOf(err)
	RegistryOpsTotal.WithLabelValues(backend, op, result).Inc()
	RegistryOpDuration.WithLabelValues(backend, op).Observe(time.Since(start).Seconds())
}

// Register registers the collector.
func Register(name string, c prometheus.Collector) {
	lock.Lock()
	defer lock.Unlock()

	pool[name] = c
}

// Unregister removes the collector.
func Unregister(name string) {
	lock.Lock()
	defer lock.Unlock()

	delete(pool, name)
}

// UnregisterAll removes all collectors.
func UnregisterAll() {
	lock.Lock()
	defer lock.Unlock()

	for k := range pool {
		delete(pool, k)
	}
}

// Get looks up a metrics instance by ID.
func Get(name string) prometheus.Collector {
	lock.RLock()
	defer lock.RUnlock()

	return pool[name]
}

// GetAll returns a copy of all registered collectors.
func GetAll() map[string]prometheus.Collector {
	lock.RLock()
	defer lock.RUnlock()

	out := make(map[string]prometheus.Collector, len(pool))
	maps.Copy(out, pool)

	return out
}

func init() {
	UnregisterAll()

	Register("sicky_server_requests_total", ServerRequestsTotal)
	Register("sicky_server_request_duration_seconds", ServerRequestDuration)
	Register("sicky_server_rejected_total", ServerRejectedTotal)
	Register("sicky_server_connections", ServerConnections)
	Register("sicky_server_panics_total", ServerPanicsTotal)
	Register("sicky_server_io_bytes_total", ServerIOBytesTotal)

	Register("sicky_client_requests_total", ClientRequestsTotal)
	Register("sicky_client_request_duration_seconds", ClientRequestDuration)
	Register("sicky_client_errors_total", ClientErrorsTotal)
	Register("sicky_client_stream_open_total", ClientStreamOpenTotal)

	Register("sicky_broker_publish_total", BrokerPublishTotal)
	Register("sicky_broker_publish_duration_seconds", BrokerPublishDuration)
	Register("sicky_broker_handler_total", BrokerHandlerTotal)
	Register("sicky_broker_handler_duration_seconds", BrokerHandlerDuration)
	Register("sicky_broker_subscribe_total", BrokerSubscribeTotal)
	Register("sicky_broker_connected", BrokerConnected)

	Register("sicky_job_registered_total", JobRegisteredTotal)
	Register("sicky_job_runs_total", JobRunsTotal)
	Register("sicky_job_run_duration_seconds", JobRunDuration)
	Register("sicky_job_ticks_total", JobTicksTotal)
	Register("sicky_job_running", JobRunning)

	Register("sicky_runner_submit_total", RunnerSubmitTotal)
	Register("sicky_runner_try_submit_total", RunnerTrySubmitTotal)
	Register("sicky_runner_task_runs_total", RunnerTaskRunsTotal)
	Register("sicky_runner_task_duration_seconds", RunnerTaskDuration)
	Register("sicky_runner_queue_depth", RunnerQueueDepth)
	Register("sicky_runner_inflight", RunnerInflight)
	Register("sicky_runner_workers", RunnerWorkers)

	Register("sicky_registry_ops_total", RegistryOpsTotal)
	Register("sicky_registry_op_duration_seconds", RegistryOpDuration)
	Register("sicky_registry_instances", RegistryInstances)
	Register("sicky_registry_watch_events_total", RegistryWatchEventsTotal)

	Register("sicky_infra_ops_total", InfraOpsTotal)
	Register("sicky_infra_op_duration_seconds", InfraOpDuration)
	Register("sicky_infra_up", InfraUp)
	Register("sicky_infra_init_total", InfraInitTotal)

	Register("sicky_manager_health_check_duration_seconds", ManagerHealthCheckDuration)
	Register("sicky_config_reloads_total", ConfigReloadsTotal)
	Register("sicky_service_starts_total", ServiceStartsTotal)

	Register("build_info", collectors.NewBuildInfoCollector())
	Register("go_collector", collectors.NewGoCollector())
	Register("process_collector", collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

/*
 * Local variables:
 * tab-width: 4
 * c-basic-offset: 4
 * End:
 * vim600: sw=4 ts=4 fdm=marker
 * vim<600: sw=4 ts=4
 */
