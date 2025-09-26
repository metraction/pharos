### Pharos CLI Parameters
 
#### Collector
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--collector.blockTimeout` | duration | `5m0s` | Block timeout while waiting on collector stream. |
| `--collector.consumerName` | string | `"single"` | Consumer name for collector group. |
| `--collector.groupName` | string | `"collector"` | Redis consumer group for collector. |
| `--collector.queueName` | string | `"scanresult"` | Collector result queue name. |
| `--collector.queueSize` | int | `100` | Internal collector queue size. |
 
#### Core / Global
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--command` | string | `"http"` | Default subcommand to run (CI convenience). |
| `--config` | string | `$HOME/.pharos.yaml` | Path to config file. |
| `-h, --help` | - | - | Show help. |
 
#### Database
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--database.driver` | string | `"postgres"` | Database driver (only postgres supported). |
| `--database.dsn` | string | `postgres://postgres:postgres@localhost:5432/pharos?sslmode=disable` | Database connection string. |
 
#### Enricher
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--enrichercommon.enricherPath` | string | `enrichers` | Base directory for enrichers. |
| `--enrichercommon.uiUrl` | string | `http://localhost:3000` | UI base URL for visual enrichers. |
 
#### Prometheus Reporter
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--prometheus.auth.username` | string | - | Basic auth username. |
| `--prometheus.auth.password` | string | - | Basic auth password. |
| `--prometheus.auth.token` | string | - | Bearer/OAuth token. |
| `--prometheus.contextLabels` | string | `namespace` | Comma-separated label keys to attach as context. |
| `--prometheus.interval` | string | `3600s` | Scrape interval. |
| `--prometheus.namespace` | string | `pharos` | Metrics namespace prefix. |
| `--prometheus.pharosUrl` | string | `http://localhost:8080` | Pharos API root for task submission. |
| `--prometheus.platform` | string | `linux/amd64` | Target platform for discovered images. |
| `--prometheus.query` | string | `kube_pod_container_info` | Prometheus query for container enumeration. |
| `--prometheus.ttl` | string | `12h` | Minimum time between re-scans of same image. |
| `--prometheus.url` | string | `http://prometheus.prometheus.svc.cluster.local:9090` | Prometheus server URL. |
 
#### Publisher
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--publisher.priorityRequestQueue` | string | `priorityScantasks` | Priority task request stream. |
| `--publisher.priorityResponseQueue` | string | `priorityScanresult` | Priority task response stream. |
| `--publisher.queueSize` | int | `1000` | Internal publisher queue size. |
| `--publisher.requestQueue` | string | `scantasks` | Standard async request stream. |
| `--publisher.responseQueue` | string | `scanresult` | Standard async response stream. |
| `--publisher.timeout` | string | `300s` | Publication timeout per task batch. |
 
#### Redis
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--redis.dsn` | string | `localhost:6379` | Redis endpoint (host:port). |
 
#### Scanner
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--scanner.cacheEndpoint` | string | `redis://localhost:6379` | Cache endpoint (content / metadata reuse). |
| `--scanner.requestQueue` | string | `scantasks` | Scanner input task stream. |
| `--scanner.responseQueue` | string | `scanresult` | Scanner output result stream. |
| `--scanner.timeout` | string | `300s` | Max scan duration per task. |
 
 
These parameters allow you to configure connectivity, authentication, queueing, and runtime behavior for Pharos.
 
> **Note:** Each command line parameter can also be set using an environment variable. The environment variable name is derived by converting the parameter to uppercase, replacing dots (`.`) with underscores (`_`), and prefixing with `PHAROS_`. For example, `--database.driver` can be set with `PHAROS_DATABASE_DRIVER`.
 
### Test it
 
Run the controller with:
 
```bash
go run main.go http
```
 
- Point your browser to: http://localhost:8080/api/docs
- Submit a scan task with sync scan: http://localhost:8080/api/v1/docs#/operations/V1PostSyncScan
 
You can also use Swagger at http://localhost:8080/api/swagger
 
 
 
 