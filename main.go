package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
)

const (
	defaultMetricsInterval = 30
	defaultAddr            = ":9295"
	defaultReashScheme     = "http"
	defaultRedashHost      = "localhost"
	defaultRedashPort      = "5000"
)

const rootDoc = `<html>
<head><title>Redash Exporter</title></head>
<body>
<h1>Redash Exporter</h1>
<p><a href="/metrics">Metrics</a></p>
</body>
</html>
`

var (
	addr            = flag.String("listen-address", defaultAddr, "The address to listen HTTP requests.")
	metricsInterval = flag.Int("metricsInterval", defaultMetricsInterval, "Interval to scrape status.")
	redashScheme    = flag.String("redashScheme", defaultReashScheme, "target Redash scheme.")
	redashHost      = flag.String("redashHost", defaultRedashHost, "target Redash host.")
	redashPort      = flag.String("redashPort", defaultRedashPort, "target Redash port.")
)

var apiKey = os.Getenv("REDASH_API_KEY")

type redashStatus struct {
	DashboardsCount float64 `json:"dashboards_count"`
	DatabaseMetrics struct {
		Metrics [][]interface{} `json:"metrics"`
	} `json:"database_metrics"`
	Manager struct {
		OutdatedQueriesCount float64 `json:"outdated_queries_count,string"`
		Queues               struct {
			Celery struct {
				Size float64 `json:"size"`
			} `json:"celery"`
			Queries struct {
				Size float64 `json:"size"`
			} `json:"queries"`
			ScheduledQueries struct {
				Size float64 `json:"size"`
			} `json:"scheduled_queries"`
		} `json:"queues"`
	} `json:"manager"`
	QueriesCount            float64 `json:"queries_count"`
	QueryResultsCount       float64 `json:"query_results_count"`
	RedisUsedMemory         float64 `json:"redis_used_memory"`
	UnusedQueryResultsCount float64 `json:"unused_query_results_count"`
	RedashVersion           string  `json:"version"`
	WidgetsCount            float64 `json:"widgets_count"`
}

var infoLabels = []string{
	"redash_version",
}

var labels = []string{}

var (
	info = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_info",
			Help: "Information of Redash.",
		},
		infoLabels,
	)

	dashboardsCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_dashboards_count",
			Help: "Number of dashboards in Redash.",
		},
		labels,
	)

	queryResultsSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_query_results_size_bytes",
			Help: "Size of Redash query results.",
		},
		labels,
	)

	dbSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_db_size_bytes",
			Help: "Size of Redash database.",
		},
		labels,
	)

	outdatedQueriesCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_outdated_queries_count",
			Help: "Number of outdated queries.",
		},
		labels,
	)

	queuesCelery = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_queues_celery",
			Help: "Number of celery queues.",
		},
		labels,
	)

	queuesQueries = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_queues_queries",
			Help: "Number of query queues.",
		},
		labels,
	)

	queuesScheduledQueries = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_queues_scheduled_queries",
			Help: "Number of scheduled query queues.",
		},
		labels,
	)

	queriesCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_queries_count",
			Help: "Number of queries stored in redash.",
		},
		labels,
	)

	queryResultsCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_query_results_count",
			Help: "Number of query results.",
		},
		labels,
	)

	redisUsedMemory = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_redis_used_memory_bytes",
			Help: "Memory size used by redis in Redash.",
		},
		labels,
	)

	unusedQueryResultsCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_unused_query_results_count",
			Help: "Number of unused query results.",
		},
		labels,
	)

	widgetsCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_wigets_count",
			Help: "Number of widgets.",
		},
		labels,
	)

	activeTasksCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "redash_active_tasks",
			Help: "Active tasks count.",
		},
		labels,
	)
)

func getRedashStatus() (redashStatus, error) {
	url := *redashScheme + "://" + *redashHost + ":" + *redashPort
	endpoint := "/status.json"
	resp, e := http.Get(url + endpoint + "?api_key=" + apiKey)
	if e != nil {
		return redashStatus{}, fmt.Errorf("httpGet error : %v", e)
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		return redashStatus{}, fmt.Errorf("io read error : %v", e)
	}
	var jsonBody redashStatus
	e = json.Unmarshal(body, &jsonBody)
	if e != nil {
		return redashStatus{}, fmt.Errorf("json parse error : %v. Is api key correct?", e)
	}
	return jsonBody, nil
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(rootDoc))
}

func main() {
	flag.Parse()
	log.Info("start Redash exporter.")
	go func() {
		for {
			status, err := getRedashStatus()
			if err != nil {
				log.Error(err)
				time.Sleep(time.Duration(*metricsInterval) * time.Second)
				continue
			}
			label := prometheus.Labels{}
			infoLabel := prometheus.Labels{
				"redash_version": status.RedashVersion,
			}
			metrics := map[string]float64{}
			for _, metric := range status.DatabaseMetrics.Metrics {
				var key string
				var val float64
				for _, m := range metric {
					switch m.(type) {
					case string:
						key = m.(string)
					case float64:
						val = m.(float64)
					}
				}
				metrics[key] = val
			}
			info.With(infoLabel).Set(float64(1))
			dashboardsCount.With(label).Set(status.DashboardsCount)
			queryResultsSize.With(label).Set(metrics["Query Results Size"])
			dbSize.With(label).Set(metrics["Redash DB Size"])
			outdatedQueriesCount.With(label).Set(float64(status.Manager.OutdatedQueriesCount))
			queuesCelery.With(label).Set(status.Manager.Queues.Celery.Size)
			queuesQueries.With(label).Set(status.Manager.Queues.Queries.Size)
			queuesScheduledQueries.With(label).Set(status.Manager.Queues.ScheduledQueries.Size)
			queriesCount.With(label).Set(status.QueriesCount)
			queryResultsCount.With(label).Set(status.QueryResultsCount)
			redisUsedMemory.With(label).Set(status.RedisUsedMemory)
			unusedQueryResultsCount.With(label).Set(status.UnusedQueryResultsCount)
			widgetsCount.With(label).Set(status.WidgetsCount)

			s, e := getRedashTasks()
			if e != nil {
				log.Error(e)
			}
			//fmt.Printf("%+v\n", s)
			//fmt.Printf("%+v\n", len(s.Tasks))
			activeTasksCount.With(label).Set(float64(len(s.Tasks)))
			time.Sleep(time.Duration(*metricsInterval) * time.Second)
		}
	}()



	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(*addr, nil))
}



func getRedashTasks() (ApiAdminQueriesTasksAnswer, error) {
	var (
		jsonBody ApiAdminQueriesTasksAnswer
		url = *redashScheme + "://" + *redashHost + ":" + *redashPort + "/api/admin/queries/tasks" + "?api_key=" + apiKey
	)
	resp, e := http.Get(url)
	if e != nil {
		return jsonBody, fmt.Errorf("httpGet error : %v", e)
	}
	body, e := ioutil.ReadAll(resp.Body)
	if e != nil {
		return jsonBody, fmt.Errorf("io read error : %v", e)
	}
	e = json.Unmarshal(body, &jsonBody)
	if e != nil {
		return jsonBody, fmt.Errorf("json parse error : %v. Is api key correct?", e)
	}
	return jsonBody, nil
}

//api/admin/queries/tasks
type ApiAdminQueriesTasksAnswer struct {
	Tasks []struct {
		Scheduled    bool    `json:"scheduled,omitempty"`
		UserID       int     `json:"user_id,omitempty"`
		TaskID       string  `json:"task_id"`
		OrgID        int     `json:"org_id,omitempty"`
		StartTime    float64 `json:"start_time"`
		Worker       string  `json:"worker"`
		EnqueueTime  float64 `json:"enqueue_time,omitempty"`
		Queue        string  `json:"queue"`
		State        string  `json:"state"`
		QueryID      string  `json:"query_id,omitempty"`
		DataSourceID int     `json:"data_source_id,omitempty"`
		TaskName     string  `json:"task_name"`
		WorkerPid    int     `json:"worker_pid"`
	} `json:"tasks"`
}
