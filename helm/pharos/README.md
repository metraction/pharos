# pharos

Helm chart for Pharos

You should at least point to the url of your prometheus installation, example for rancher:
```
prometheus.url=http://rancher-monitoring-prometheus.cattle-monitoring-system.svc.cluster.local:9090
```
# Pharos Helm Chart Values Reference

This document describes the configuration options available in the `values.yaml` file for the Pharos Helm chart.  
Each value can be set in your own `values.yaml` or via `--set` on the command line.

---

## Table of Contents

- [image](#image)
- [imagePullSecrets](#imagepullsecrets)
- [ingress](#ingress)
- [service](#service)
- [redis](#redis)
- [scannerPod](#scannerpod)
- [priorityScannerPod](#priorityscannerpod)
- [externalDatabase](#externaldatabase)
- [postgres](#postgres)
  - [postgres.image](#postgresimage)
  - [postgres.serviceAccount](#postgresserviceaccount)
  - [postgres.startupProbe / livenessProbe / readinessProbe](#postgresprobes)
  - [postgres.storage](#postgresstorage)
  - [postgres.podSecurityContext](#postgrespodsecuritycontext)
  - [postgres.securityContext](#postgressecuritycontext)
  - [postgres.service](#postgresservice)
  - [postgres.auth](#postgresauth)
- [externalRedis](#externalredis)
- [serviceAccount](#serviceaccount)
- [role](#role)
- [controller](#controller)
  - [controller.collector](#controllercollector)
  - [controller.publisher](#controllerpublisher)
- [prometheus](#prometheus)
  - [prometheus.auth](#prometheusauth)
  - [prometheus.authFromSecret](#prometheusauthfromsecret)
- [enrichers](#enrichers)
  - [enrichers.mappers](#enrichersmappers)
- [caCertificates](#cacertificates)
- [alerting](#alerting)
  - [alerting.route](#alertingroute)
  - [alerting.receivers](#alertingreceivers)
- [jiralert](#jiralert)
  - [jiralert.config](#jiralertconfig)

---

## image

| Key        | Type   | Description                       |
|------------|--------|-----------------------------------|
| registry   | string | Registry for pharos image         |
| repository | string | Repository for pharos image       |
| pullPolicy | string | Image pull policy (`Always`, `IfNotPresent`, `Never`) |

---

## imagePullSecrets

| Key  | Type   | Description   |
|------|--------|---------------|
| name | string | Secret name   |

---

## ingress

| Key           | Type    | Description           |
|---------------|---------|-----------------------|
| enabled       | boolean | Enable ingress        |
| host          | string  | Ingress host          |
| className     | string  | Ingress class name    |
| tls           | boolean | Enable TLS            |
| tlsSecretName | string  | TLS secret name       |
| annotations   | object  | Ingress annotations (key-value pairs) |

---

## service

| Key  | Type    | Description   |
|------|---------|---------------|
| port | integer | Service port  |

---

## redis

| Key      | Type             | Description                                 |
|----------|------------------|---------------------------------------------|
| enabled  | boolean          | Deploy Redis                                |
| auth     | boolean/object   | Enable Redis AUTH or auth config            |
| replicas | integer          | Replica count                               |

---

## scannerPod / priorityScannerPod

| Key     | Type    | Description           |
|---------|---------|-----------------------|
| enabled | boolean | Enable scanner pod    |

---

## externalDatabase

| Key   | Type   | Description                                         |
|-------|--------|-----------------------------------------------------|
| secret| string | Secret name containing DSN for external database    |

---

## postgres

| Key                | Type    | Description                        |
|--------------------|---------|------------------------------------|
| enabled            | boolean | Deploy PostgreSQL                  |
| [image](#postgresimage)              | object  | Image settings for PostgreSQL      |
| dataDir            | string  | Data directory                     |
| pgDir              | string  | PG directory                       |
| [serviceAccount](#postgresserviceaccount)     | object  | Service account settings           |
| [startupProbe](#postgresprobes)       | object  | Startup probe settings             |
| [livenessProbe](#postgresprobes)      | object  | Liveness probe settings            |
| [readinessProbe](#postgresprobes)     | object  | Readiness probe settings           |
| resources          | object  | Resource requests/limits           |
| [storage](#postgresstorage)           | object  | Storage settings                   |
| podManagementPolicy| string  | Pod management policy              |
| updateStrategyType | string  | Update strategy type               |
| [podSecurityContext](#postgrespodsecuritycontext) | object  | Pod security context               |
| [securityContext](#postgressecuritycontext)    | object  | Security context                   |
| [service](#postgresservice)           | object  | Service settings for PostgreSQL    |
| [auth](#postgresauth)                 | object  | Authentication settings            |

### postgres.image

| Key        | Type   | Description   |
|------------|--------|---------------|
| registry   | string | Registry      |
| repository | string | Repository    |
| pullPolicy | string | Pull policy   |
| tag        | string | Image tag     |

### postgres.serviceAccount

| Key        | Type    | Description   |
|------------|---------|---------------|
| create     | boolean | Create SA     |
| annotations| object  | Annotations   |
| name       | string  | Name          |

### postgresProbes

_Applies to `startupProbe`, `livenessProbe`, `readinessProbe`_

| Key                 | Type    | Description   |
|---------------------|---------|---------------|
| enabled             | boolean | Enable probe  |
| initialDelaySeconds | integer | Initial delay |
| timeoutSeconds      | integer | Timeout       |
| failureThreshold    | integer | Failures      |
| successThreshold    | integer | Successes     |
| periodSeconds       | integer | Period        |

### postgres.storage

| Key         | Type    | Description   |
|-------------|---------|---------------|
| volumeName  | string  | Volume name   |
| requestedSize| string | PVC size      |
| className   | string  | Storage class |
| accessModes | array   | Access modes  |
| keepPvc     | boolean | Keep PVC      |
| annotations | object  | Annotations   |
| labels      | object  | Labels        |

### postgres.podSecurityContext

| Key              | Type    | Description   |
|------------------|---------|---------------|
| fsGroup          | integer | FS group      |
| supplementalGroups| array  | Supplemental groups |

### postgres.securityContext

| Key                    | Type    | Description   |
|------------------------|---------|---------------|
| allowPrivilegeEscalation| boolean| Priv escalation|
| privileged             | boolean | Privileged    |
| readOnlyRootFilesystem | boolean | Read-only FS  |
| runAsNonRoot           | boolean | Run as non-root|
| runAsGroup             | integer | Run as group  |
| runAsUser              | integer | Run as user   |
| capabilities           | object  | Capabilities  |

### postgres.service

| Key                     | Type    | Description   |
|-------------------------|---------|---------------|
| type                    | string  | Service type  |
| port                    | integer | Service port  |
| nodePort                | integer | NodePort      |
| clusterIP               | string  | ClusterIP     |
| loadBalancerIP          | string  | LB IP         |
| loadBalancerSourceRanges| array   | LB source CIDRs|
| annotations             | object  | Annotations   |
| labels                  | object  | Labels        |

### postgres.auth

| Key             | Type   | Description   |
|-----------------|--------|---------------|
| postgresPassword| string | Password      |
| username        | string | Username      |
| password        | string | Password      |
| database        | string | Database      |

---

## externalRedis

| Key  | Type    | Description           |
|------|---------|-----------------------|
| host | string  | External Redis host   |
| port | integer | External Redis port   |

---

## serviceAccount

| Key    | Type    | Description           |
|--------|---------|-----------------------|
| create | boolean | Create ServiceAccount |

---

## role

| Key    | Type    | Description      |
|--------|---------|------------------|
| create | boolean | Create Role      |

---

## controller

| Key       | Type    | Description           |
|-----------|---------|-----------------------|
| replicas  | integer | Controller replicas   |
| [collector](#controllercollector) | object  | Collector settings    |
| [publisher](#controllerpublisher) | object  | Publisher settings    |

### controller.collector

| Key      | Type    | Description           |
|----------|---------|-----------------------|
| queueSize| integer | Collector queue size  |

### controller.publisher

| Key      | Type    | Description           |
|----------|---------|-----------------------|
| queueSize| integer | Publisher queue size  |

---

## prometheus

| Key           | Type    | Description                       |
|---------------|---------|-----------------------------------|
| url           | string  | Prometheus server URL             |
| query         | string  | Prometheus query to get images    |
| interval      | string  | Poll interval (duration)          |
| contextLabels | string  | Comma separated context labels     |
| ttl           | string  | Scan result TTL (duration)        |
| [auth](#prometheusauth)          | object  | Prometheus authentication         |
| [authFromSecret](#prometheusauthfromsecret) | object  | Auth from secret                  |

### prometheus.auth

| Key      | Type   | Description   |
|----------|--------|---------------|
| username | string | Username      |
| password | string | Password      |
| token    | string | Bearer token  |

### prometheus.authFromSecret

| Key         | Type    | Description   |
|-------------|---------|---------------|
| enabled     | boolean | Enable        |
| existingSecret| string| Secret name   |
| usernameKey | string  | Username key  |
| passwordKey | string  | Password key  |
| tokenKey    | string  | Token key     |

---

## enrichers

| Key       | Type    | Description                                 |
|-----------|---------|---------------------------------------------|
| configMap | string  | ConfigMap name for enrichers                |
| config    | string  | Path to base enricher config                |
| [mappers](#enrichersmappers)   | object  | Mapper templates/files                      |
| uiUrl     | string  | URL to Pharos UI for visual enrichers       |

### enrichers.mappers

| Key   | Type   | Description   |
|-------|--------|---------------|
| hbs   | object | Inline handlebars templates (filename: string) |
| files | object | External mapper files (filename: string)       |

---

## caCertificates

| Key            | Type    | Description                          |
|----------------|---------|--------------------------------------|
| configMapEnabled| boolean| Enable CA certs ConfigMap creation   |
| enabled        | boolean | Mount CA certs into reporter pod     |
| configMapName  | string  | Name of existing CA certs ConfigMap  |

---

## alerting

| Key      | Type    | Description                                 |
|----------|---------|---------------------------------------------|
| [route](#alertingroute)    | object  | Alert routing (Prometheus Alertmanager style)|
| [receivers](#alertingreceivers)| array   | Alert receivers                             |

### alerting.route

| Key         | Type    | Description   |
|-------------|---------|---------------|
| group_by    | array   | Group by labels|
| continue    | boolean | Continue      |
| receiver    | string  | Default receiver name |
| child_routes| array   | Nested routes |

### alerting.receivers

| Key            | Type    | Description   |
|----------------|---------|---------------|
| name           | string  | Receiver name |
| webhook_configs| array   | Webhook configs (see below) |

#### alerting.receivers[].webhook_configs

| Key           | Type    | Description   |
|---------------|---------|---------------|
| url           | string  | Webhook URL   |
| send_resolved | boolean | Send resolved notifications |

---

## jiralert

| Key          | Type    | Description                             |
|--------------|---------|-----------------------------------------|
| enabled      | boolean | Enable Jiralert deployment              |
| extraArgs    | array   | Extra CLI args for jiralert             |
| [config](#jiralertconfig)       | object  | Jiralert configuration file contents     |
| issueTemplate| string  | Inline JIRA template definitions        |

### jiralert.config

| Key      | Type    | Description   |
|----------|---------|---------------|
| template | string  | Template file path |
| defaults | object  | Global default settings |
| receivers| array   | Receiver definitions (name, project) |

---

**Note:**  
For more details and examples, see the [values.yaml](./values.yaml) file in the chart.
## Enabling Jira Alerting

Please follow the example in [./values-jiralert-example.yaml](./values-jiralert-example.yaml) to enable Jira alerting.
This will install the jiralert component in your cluster, and configure pharos to send alerts to it.
