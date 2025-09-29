# pharos

Helm chart for pharos

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | postgres(postgresql) | ~12.5.7 |
| https://charts.bitnami.com/bitnami | redis(redis) | ~17.0.0 |
| https://prometheus-community.github.io/helm-charts | jiralert | 1.8.1 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| alerting.receivers[0] | object | `{"name":"default"}` | list of receivers to receive alerts |
| alerting.route | object | `{"child_routes":[],"continue":true,"group_by":["..."],"receiver":"default"}` | Alerting configuration this basically follows the prometheus alertmanager configuration |
| caCertificates.configMapEnabled | bool | `false` | Enable CA certificates configMap |
| caCertificates.configMapName | string | `"ca-certificates"` | ConfigMap name for CA certificates, bring your own if configMapEnabled is true |
| caCertificates.enabled | bool | `false` | Enable CA certificates in the reporter pod |
| controller.collector.queueSize | int | `100` | Queue size for the collector |
| controller.publisher.queueSize | int | `1000` | Queue size for the publisher |
| controller.replicas | int | `1` | Number of replicas for the controller |
| enrichers | object | `{"config":"enrichers/enricher.yaml","configMap":"pharos-enrichers","mappers":{"files":{"eos.yaml":"files/eos.yaml"},"hbs":{"eos_v1.hbs":"distro: {{ .payload.Image.DistroName }}\nversion: {{ .payload.Image.DistroVersion }}\neos: {{ index .meta.eos .payload.Image.DistroName | filter \"version\" \"matchWildcard\" .payload.Image.DistroVersion | map \"field\" \"eos\" | first }}\n"}},"uiUrl":""}` | Enrichers configuration |
| externalRedis | object | `{"host":"localhost","port":6379}` | External Redis configuration (used when redis.enabled=false) |
| image.pullPolicy | string | `"Always"` | pull policy for pharos-image |
| image.registry | string | `"ghcr.io"` | registry for pharos-image |
| image.repository | string | `"metraction/pharos"` | repository for pharos-image |
| imagePullSecrets | list | `[]` | list of imagePullSecrets to use. These secrets are also used to get the images to scan. |
| ingress.enabled | bool | `false` | Enable ingress for pharos |
| jiralert.enabled | bool | `false` | Enable JIRA alerting |
| postgres | object | `{"auth":{"existingSecret":"postgres-connection"},"enabled":true,"primary":{"persistence":{"enabled":true,"size":"1Gi"}}}` | PostgreSQL configuration |
| postgres.auth | object | `{"existingSecret":"postgres-connection"}` | PostgreSQL authentication |
| postgres.auth.existingSecret | string | `"postgres-connection"` | Use an existing secret for PostgreSQL connection |
| postgres.enabled | bool | `true` | Enable PostgreSQL deployment |
| priorityScannerPod.enabled | bool | `false` | Enable the scanner pod, only needed if you are not using direct scan |
| prometheus | object | `{"auth":{"password":"","token":"","username":""},"authFromSecret":{"enabled":false,"existingSecret":"","passwordKey":"password","tokenKey":"","usernameKey":"username"},"contextLabels":"namespace","interval":"10m","query":"kube_pod_container_info{}","ttl":"12h","url":"http://prometheus.prometheus.svc.cluster.local:9090"}` | Prometheus configuration for scanning images |
| prometheus.auth | object | `{"password":"","token":"","username":""}` | Authentication for Prometheus |
| prometheus.auth.password | string | `""` | Password for Prometheus authentication |
| prometheus.auth.token | string | `""` | Token for Prometheus authentication |
| prometheus.auth.username | string | `""` | Username for Prometheus authentication |
| prometheus.authFromSecret.enabled | bool | `false` | Enable authentication from an existing secret |
| prometheus.authFromSecret.existingSecret | string | `""` | Use an existing secret for Prometheus authentication |
| prometheus.authFromSecret.passwordKey | string | `"password"` | Key in the secret for the password |
| prometheus.authFromSecret.tokenKey | string | `""` | Key in the secret for the token |
| prometheus.authFromSecret.usernameKey | string | `"username"` | Key in the secret for the username |
| prometheus.contextLabels | string | `"namespace"` | Context labels to add to the Prometheus context |
| prometheus.interval | string | `"10m"` | Interval for scanning images |
| prometheus.query | string | `"kube_pod_container_info{}"` | Prometheus query to get the images to scan |
| prometheus.ttl | string | `"12h"` | Time to live for the scan results, defaults to 12 hours |
| prometheus.url | string | `"http://prometheus.prometheus.svc.cluster.local:9090"` | Url of the Prometheus server |
| redis.auth | object | `{"enabled":false}` | Redis authentication |
| redis.auth.enabled | bool | `false` | Enable Redis authentication |
| redis.enabled | bool | `true` | Enable Redis deployment |
| redis.replica | object | `{"replicaCount":1}` | Redis replica configuration |
| redis.replica.replicaCount | int | `1` | Number of Redis replicas to deploy |
| role | object | `{"create":true}` | Role configuration - needed to read ImagePullSecrets |
| scannerPod.enabled | bool | `false` | Enable the scanner pod, only neeed if you are not using direct scan |
| service.port | int | `8080` | port for the service |
| serviceAccount | object | `{"create":true}` | Service account configuration - needed to read ImagePullSecrets |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)

## Enabling Jira Alerting

Please follow the example in [./values-jiralert-example.yaml](./values-jiralert-example.yaml) to enable Jira alerting.
This will install the jiralert component in your cluster, and configure pharos to send alerts to it.
