# pharos

Helm chart for Pharos

You should at least point to the url of your prometheus installation, example for rancher:
```
prometheus.url=http://rancher-monitoring-prometheus.cattle-monitoring-system.svc.cluster.local:9090
```
# Pharos Helm Chart Values Reference

This document describes the configuration options available in the `values.yaml` file for the Pharos Helm chart.  
Each value can be set in your own `values.yaml` or via `--set` on the command line.

## <a name="values.schema.json"></a>Pharos Helm Chart Values

Referenced by Schema ID: <a href="#"></a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `alerting.receivers` | `object[]` | [Alert receivers](#c5886cd4d1a770ae982d23dd96b17821e133011cd1e9eeab72bd50773987c1f9) |  |  |  |
| `alerting.route` | `object` | [alerting.route](#7e23563bda966e1a64beb0aefc307dee43538b8c60cab9a6e136029579407e3b) |  |  |  |
| `caCertificates.configMapEnabled` | `boolean` | Enable CA certs ConfigMap creation |  |  |  |
| `caCertificates.configMapName` | `string` | Name of existing CA certs ConfigMap |  |  |  |
| `caCertificates.enabled` | `boolean` | Mount CA certs into reporter pod |  |  |  |
| `controller.collector` | `object` | [controller.collector](#16073839309359a9ec90b729c1fb56d8b2f9458c566406abecb7ad995e51b637) |  |  |  |
| `controller.publisher` | `object` | [controller.publisher](#401d33198791f650ad476ed559af6c56b90aef91959490a41c9da98e92a4d2be) |  |  |  |
| `controller.replicas` | `integer` | Controller replicas |  |  |  |
| `controller.resources` | `object` |  |  |  |  |
| `enrichers.config` | `string` | Path to base enricher config |  |  |  |
| `enrichers.configMap` | `string` | ConfigMap name for enrichers |  |  |  |
| `enrichers.definitions` | `object` | Definitions of enrichers |  |  |  |
| `enrichers.order` | `string[]` | Order of enrichers |  |  |  |
| `enrichers.uiUrl` | `string` | URL to Pharos UI for visual enrichers (empty disables) |  |  |  |
| `externalDatabase.secret` | `string` | Secret name containing DSN for external database |  |  |  |
| `externalRedis.host` | `string` | External Redis host |  |  |  |
| `externalRedis.port` | `integer` | External Redis port |  |  |  |
| `image.pullPolicy` | `string` | Image pull policy |  |  |  |
| `image.registry` | `string` | Registry for pharos image |  |  |  |
| `image.repository` | `string` | Repository for pharos image |  |  |  |
| `imagePullSecrets[].name` | `string` | Secret name |  |  |  |
| `ingress.annotations` | `object` | Ingress annotations |  |  |  |
| `ingress.className` | `string` | Ingress class name |  |  |  |
| `ingress.enabled` | `boolean` | Enable ingress |  |  |  |
| `ingress.host` | `string` | Ingress host |  |  |  |
| `ingress.tls` | `boolean` | Enable TLS |  |  |  |
| `ingress.tlsSecretName` | `string` | TLS secret name |  |  |  |
| `jiralert.config` | `object` | [Jiralert configuration file contents](#0b9d1b3ee17bbb3cb8519d7169bc1511a9ec0296b00de5edd27884bb56b83fc9) |  |  |  |
| `jiralert.enabled` | `boolean` | Enable Jiralert deployment |  |  |  |
| `jiralert.extraArgs` | `string[]` | Extra CLI args for jiralert |  |  |  |
| `jiralert.issueTemplate` | `string` | Inline JIRA template definitions |  |  |  |
| `postgres.auth` | `object` | [postgres.auth](#cf1e9b8734bafafb2bed2f380e1d58c26b9000faea599573002fb34b51b15941) |  |  |  |
| `postgres.dataDir` | `string` |  |  |  |  |
| `postgres.enabled` | `boolean` | Deploy PostgreSQL |  |  |  |
| `postgres.image` | `object` | [postgres.image](#df162de5f4e8b01560a3f997cd8f6f24efbec775a04fb8133d276ac6c02c14eb) |  |  |  |
| `postgres.livenessProbe` | `object` | [postgres.livenessProbe](#65d930d1ae28735fdefc0d8cdb21f975cb24cddc38b5caf4072b5e8a639efd02) |  |  |  |
| `postgres.pgDir` | `string` |  |  |  |  |
| `postgres.podManagementPolicy` | `string` |  |  |  |  |
| `postgres.podSecurityContext` | `object` | [postgres.podSecurityContext](#af2a21e0c49f7b5f19356c5c16782524e731e3791ccc5f46a59d2542a0a5fa34) |  |  |  |
| `postgres.readinessProbe` | `object` | [postgres.readinessProbe](#65d930d1ae28735fdefc0d8cdb21f975cb24cddc38b5caf4072b5e8a639efd02) |  |  |  |
| `postgres.resources` | `object` |  |  |  |  |
| `postgres.securityContext` | `object` | [postgres.securityContext](#95d8987ae5a683c53ae5932928476387baab9b5176bd282f64c3950add945bae) |  |  |  |
| `postgres.service` | `object` | [postgres.service](#58a6554a3336c2a88493003cdda6e2d6649146fe170659498af8a6aef17ab0d9) |  |  |  |
| `postgres.serviceAccount` | `object` | [postgres.serviceAccount](#1e83abfeea4699b49fc661613d500238a433ecfa9c3dbaccc3c0705d761ceddb) |  |  |  |
| `postgres.startupProbe` | `object` | [postgres.startupProbe](#65d930d1ae28735fdefc0d8cdb21f975cb24cddc38b5caf4072b5e8a639efd02) |  |  |  |
| `postgres.storage` | `object` | [postgres.storage](#7ca6179c6bc9a79dd310c921290e932791d6a13d7bcc221fb85efcbf3e369bae) |  |  |  |
| `postgres.updateStrategyType` | `string` |  |  |  |  |
| `priorityScannerPod.enabled` | `boolean` | Enable priority scanner pod |  |  |  |
| `prometheus.auth` | `object` | [prometheus.auth](#8866942597328b528486c94b56a01b1642bd6ec870bb319085cdcab95febbda9) |  |  |  |
| `prometheus.authFromSecret` | `object` | [prometheus.authFromSecret](#863fed7e0545f61d05589aac4db751a72f0dd45dc19ffcf8a3e14a3286071646) |  |  |  |
| `prometheus.contextLabels` | `string` | Comma separated labels to add to context |  |  |  |
| `prometheus.interval` | `string` | Poll interval (duration) |  |  |  |
| `prometheus.query` | `string` | Prometheus query to get images |  |  |  |
| `prometheus.ttl` | `string` | Scan result TTL (duration) |  |  |  |
| `prometheus.url` | `string` | Prometheus server URL |  |  |  |
| `redis.auth` | `unknown` | Enable Redis AUTH or auth config |  |  |  |
| `redis.enabled` | `boolean` | Deploy Redis |  |  |  |
| `redis.persistentVolume` | `object` | [redis.persistentVolume](#0d0e9d650424ed8ec356e26e2bb1e93f414fb82db4b56c55bbb6b63dbaaa1906) |  |  |  |
| `redis.replicas` | `integer` | Replica count |  |  |  |
| `role.create` | `boolean` | Create Role |  |  |  |
| `scannerPod.enabled` | `boolean` | Enable scanner pod |  |  |  |
| `scheduler.resources` | `object` |  |  |  |  |
| `service.port` | `integer` | Service port |  |  |  |
| `serviceAccount.create` | `boolean` | Create ServiceAccount |  |  |  |



## <a name="7e23563bda966e1a64beb0aefc307dee43538b8c60cab9a6e136029579407e3b"></a>alerting.route

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `child_routes` | `object[]` | Nested routes |  |  |  |
| `continue` | `boolean` | Continue processing child routes |  |  |  |
| `group_by` | `string[]` | Group by labels |  |  |  |
| `receiver` | `string` | Default receiver name |  |  |  |



## <a name="c5886cd4d1a770ae982d23dd96b17821e133011cd1e9eeab72bd50773987c1f9"></a>Alert receivers

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `name` | `string` | Receiver name |  |  |  |
| `webhook_configs[].send_resolved` | `boolean` | Send resolved notifications |  |  |  |
| `webhook_configs[].url` | `string` | Webhook URL |  |  |  |



## <a name="0d0e9d650424ed8ec356e26e2bb1e93f414fb82db4b56c55bbb6b63dbaaa1906"></a>redis.persistentVolume

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `size` | `string` | Persistent volume size |  |  |  |



## <a name="0b9d1b3ee17bbb3cb8519d7169bc1511a9ec0296b00de5edd27884bb56b83fc9"></a>Jiralert configuration file contents

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `defaults` | `object` | Global default settings |  |  |  |
| `receivers[].name` | `string` |  |  |  |  |
| `receivers[].project` | `string` |  |  |  |  |
| `template` | `string` | Template file path |  |  |  |



## <a name="65d930d1ae28735fdefc0d8cdb21f975cb24cddc38b5caf4072b5e8a639efd02"></a>postgres.readinessProbe

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `enabled` | `boolean` |  |  |  |  |
| `failureThreshold` | `integer` |  |  |  |  |
| `initialDelaySeconds` | `integer` |  |  |  |  |
| `periodSeconds` | `integer` |  |  |  |  |
| `successThreshold` | `integer` |  |  |  |  |
| `timeoutSeconds` | `integer` |  |  |  |  |



## <a name="65d930d1ae28735fdefc0d8cdb21f975cb24cddc38b5caf4072b5e8a639efd02"></a>postgres.startupProbe

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `enabled` | `boolean` |  |  |  |  |
| `failureThreshold` | `integer` |  |  |  |  |
| `initialDelaySeconds` | `integer` |  |  |  |  |
| `periodSeconds` | `integer` |  |  |  |  |
| `successThreshold` | `integer` |  |  |  |  |
| `timeoutSeconds` | `integer` |  |  |  |  |



## <a name="7ca6179c6bc9a79dd310c921290e932791d6a13d7bcc221fb85efcbf3e369bae"></a>postgres.storage

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `accessModes` | `string[]` |  |  |  |  |
| `annotations` | `object` |  |  |  |  |
| `className` | `unknown` |  |  |  |  |
| `keepPvc` | `boolean` |  |  |  |  |
| `labels` | `object` |  |  |  |  |
| `requestedSize` | `string` |  |  |  |  |
| `volumeName` | `string` |  |  |  |  |



## <a name="cf1e9b8734bafafb2bed2f380e1d58c26b9000faea599573002fb34b51b15941"></a>postgres.auth

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `database` | `string` |  |  |  |  |
| `password` | `string` |  |  |  |  |
| `postgresPassword` | `string` |  |  |  |  |
| `username` | `string` |  |  |  |  |



## <a name="df162de5f4e8b01560a3f997cd8f6f24efbec775a04fb8133d276ac6c02c14eb"></a>postgres.image

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `pullPolicy` | `string` |  |  |  |  |
| `registry` | `string` |  |  |  |  |
| `repository` | `string` |  |  |  |  |
| `tag` | `string` |  |  |  |  |



## <a name="65d930d1ae28735fdefc0d8cdb21f975cb24cddc38b5caf4072b5e8a639efd02"></a>postgres.livenessProbe

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `enabled` | `boolean` |  |  |  |  |
| `failureThreshold` | `integer` |  |  |  |  |
| `initialDelaySeconds` | `integer` |  |  |  |  |
| `periodSeconds` | `integer` |  |  |  |  |
| `successThreshold` | `integer` |  |  |  |  |
| `timeoutSeconds` | `integer` |  |  |  |  |



## <a name="af2a21e0c49f7b5f19356c5c16782524e731e3791ccc5f46a59d2542a0a5fa34"></a>postgres.podSecurityContext

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `fsGroup` | `integer` |  |  |  |  |
| `supplementalGroups` | `integer[]` |  |  |  |  |



## <a name="58a6554a3336c2a88493003cdda6e2d6649146fe170659498af8a6aef17ab0d9"></a>postgres.service

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `annotations` | `object` |  |  |  |  |
| `clusterIP` | `unknown` |  |  |  |  |
| `labels` | `object` |  |  |  |  |
| `loadBalancerIP` | `unknown` |  |  |  |  |
| `loadBalancerSourceRanges` | `string[]` |  |  |  |  |
| `nodePort` | `unknown` |  |  |  |  |
| `port` | `integer` |  |  |  |  |
| `type` | `string` |  |  |  |  |



## <a name="95d8987ae5a683c53ae5932928476387baab9b5176bd282f64c3950add945bae"></a>postgres.securityContext

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `allowPrivilegeEscalation` | `boolean` |  |  |  |  |
| `capabilities.drop` | `string[]` |  |  |  |  |
| `privileged` | `boolean` |  |  |  |  |
| `readOnlyRootFilesystem` | `boolean` |  |  |  |  |
| `runAsGroup` | `integer` |  |  |  |  |
| `runAsNonRoot` | `boolean` |  |  |  |  |
| `runAsUser` | `integer` |  |  |  |  |



## <a name="1e83abfeea4699b49fc661613d500238a433ecfa9c3dbaccc3c0705d761ceddb"></a>postgres.serviceAccount

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `annotations` | `object` |  |  |  |  |
| `create` | `boolean` |  |  |  |  |
| `name` | `string` |  |  |  |  |



## <a name="401d33198791f650ad476ed559af6c56b90aef91959490a41c9da98e92a4d2be"></a>controller.publisher

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `queueSize` | `integer` | Publisher queue size |  |  |  |



## <a name="16073839309359a9ec90b729c1fb56d8b2f9458c566406abecb7ad995e51b637"></a>controller.collector

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `queueSize` | `integer` | Collector queue size |  |  |  |



## <a name="8866942597328b528486c94b56a01b1642bd6ec870bb319085cdcab95febbda9"></a>prometheus.auth

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `password` | `string` | Basic auth password |  |  |  |
| `token` | `string` | Bearer token |  |  |  |
| `username` | `string` | Basic auth username |  |  |  |



## <a name="863fed7e0545f61d05589aac4db751a72f0dd45dc19ffcf8a3e14a3286071646"></a>prometheus.authFromSecret

Referenced by Schema ID: <a href="#values.schema.json">values.schema.json</a>

| Key | Type | Description | Default | Examples | Extra |
|-----|------|-------------|---------|----------|-------|
| `enabled` | `boolean` | Enable reading auth from secret |  |  |  |
| `existingSecret` | `string` | Secret name |  |  |  |
| `passwordKey` | `string` | Key for password |  |  |  |
| `tokenKey` | `string` | Key for token |  |  |  |
| `usernameKey` | `string` | Key for username |  |  |  |



**Note:**  
For more details and examples, see the [values.yaml](./values.yaml) file in the chart.
## Enabling Jira Alerting

Please follow the example in [./values-jiralert-example.yaml](./values-jiralert-example.yaml) to enable Jira alerting.
This will install the jiralert component in your cluster, and configure pharos to send alerts to it.
