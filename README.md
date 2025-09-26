# Pharos

## Installation

### Helm Chart

See [helm chart](./helm/pharos/) how to install pharos via helm chart.


### Grafana

Import the [dashboard](./grafana/pharos-dashboard.json) into your Grafana installation

Create datasources for Pharos:

- Type: Infinity
- Name: The namespace where it runs in
- Base URL: The URL defined by the [ingres in values.yaml](./helm/pharos/values.yaml)

## Run and test it.

### How run Pharos from the command line

`go run main.go`

#### Positional Commands

Available Commands:

- **`completion`**: Generate the autocompletion script for the specified shell.
- **`help`**: Help about any command.
- **`http`**: Starts an HTTP server to accept image scan requests.
- **`prometheus-reporter`**: Report images from Prometheus to Pharos.
- **`scanner`**: Run the Pharos scanner.

The following command line parameters are available:

- **`--config <path>`**: Path to the configuration file (default: `$HOME/.pharos.yaml`).
- **`--help`, `-h`**: Display help for the root command.

##### Database Parameters

- **`--database.driver <driver>`**: Database driver (default: `postgres`).
- **`--database.dsn <dsn>`**: Database DSN/connection string (default: `postgres://postgres:postgres@localhost:5432/pharos?sslmode=disable`).

##### Redis & Queue Parameters

- **`--redis.dsn <dsn>`**: Redis address (default: `localhost:6379`).
- **`--publisher.requestQueue <name>`**: Redis stream for async requests (default: `scantasks`).
- **`--publisher.responseQueue <name>`**: Redis stream for async responses (default: `scanresult`).
- **`--publisher.priorityRequestQueue <name>`**: Redis stream for sync requests (default: `priorityScantasks`).
- **`--publisher.priorityResponseQueue <name>`**: Redis stream for sync responses (default: `priorityScanresult`).
- **`--publisher.timeout <duration>`**: Publisher timeout (default: `300s`).

##### Scanner Parameters

- **`--scanner.cacheEndpoint <url>`**: Scanner cache endpoint (default: `redis://localhost:6379`).
- **`--scanner.requestQueue <name>`**: Redis stream for requests (default: `scantasks`).
- **`--scanner.responseQueue <name>`**: Redis stream for responses (default: `scanresult`).
- **`--scanner.timeout <duration>`**: Scanner timeout (default: `300s`).

##### Collector Parameters

- **`--collector.blockTimeout <duration>`**: Redis stream block timeout for async responses (default: `5m0s`).
- **`--collector.consumerName <name>`**: Redis stream consumer name (default: `single`).
- **`--collector.groupName <name>`**: Redis stream group name (default: `collector`).
- **`--collector.messageCount <count>`**: Redis stream message count (default: `100`).
- **`--collector.queueName <name>`**: Redis stream queue name (default: `scanresult`).

##### Mapper Parameters

- **`--mapper.basePath <path>`**: Base path for the mappers (default: `cmd/kodata/enrichers`).

##### Prometheus Reporter Parameters

- **`--prometheus.auth.username <username>`**: Username for Prometheus authentication.
- **`--prometheus.auth.password <password>`**: Password for Prometheus authentication.
- **`--prometheus.auth.token <token>`**: Token for Prometheus authentication.
- **`--prometheus.contextLabels <labels>`**: Labels to add to the Prometheus context (default: `namespace`).
- **`--prometheus.interval <duration>`**: Interval for scraping Prometheus metrics (default: `3600s`).
- **`--prometheus.namespace <namespace>`**: Namespace for Prometheus metrics (default: `pharos`).
- **`--prometheus.pharosUrl <url>`**: Root URL of the Pharos server (default: `http://localhost:8080`).
- **`--prometheus.platform <platform>`**: Platform for metrics collection (default: `linux/amd64`).
- **`--prometheus.query <query>`**: Query for fetching metrics (default: `kube_pod_container_info`).
- **`--prometheus.ttl <duration>`**: Time to live for scan results (default: `12h`).
- **`--prometheus.url <url>`**: URL of the Prometheus server (default: `http://prometheus.prometheus.svc.cluster.local:9090`).

These parameters allow you to configure connectivity, authentication, queueing, and runtime behavior for Pharos.

> **Note:** Each command line parameter can also be set using an environment variable. The environment variable name is derived by converting the parameter to uppercase, replacing dots (`.`) with underscores (`_`), and prefixing with `PHAROS_`. For example, `--database.driver` can be set with `PHAROS_DATABASE_DRIVER`.

### Test it

Run the controller with: 

```bash
go run main.go http
```

- Point your browser to: http://localhost:8080/api/docs
- Submit a scan task with sync scan: http://localhost:8080/api/docs#/operations/SyncScan (you can use the simple example provided)

```bash
curl --request POST \
  --url http://localhost:8080/api/pharosscantask/syncscan \
  --header 'Accept: application/json' \
  --header 'Content-Type: application/json' \
  --data '{
"imageSpec": {
"image": "redis:latest"
}
}'
```

- The Sanner returns the scan result and saves to the database.

- Do an async scan: http://localhost:8080/api/docs#/operations/AsyncScan

```bash
curl --request POST \
  --url http://localhost:8080/api/pharosscantask/asyncsyncscan \
  --header 'Accept: application/json' \
  --header 'Content-Type: application/json' \
  --data '{
"imageSpec": {
"image": "nginx:latest"
}
}'
```

- Get all Images: http://localhost:8080/api/docs#/operations/GetAllImages (Without vulnerabilities)
- Get Image with all Details (Vulnerabilities, Packages and Findings from the datase): http://localhost:8080/api/docs#/operations/Getimage (Take any ImageId from the previous step)

```bash
curl --request GET \
  --url http://localhost:8080/api/pharosimagemeta/sha256:1e5f3c5b981a9f91ca91cf13ce87c2eedfc7a083f4f279552084dd08fc477512 \
  --header 'Accept: application/json'
```

> ImageId is not the digest, but some internal id we get from the scanner. So you have to find it by getting all images. (we will provide a function later.)

You can also use Swagger at http://localhost:8080/api/swagger

