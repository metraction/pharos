# Pharos: Automated Container Image Security and Compliance Platform

## Abstract

Pharos is an open-source platform designed to automate the security scanning, vulnerability management, and compliance reporting of container images in modern DevOps environments. It integrates with CI/CD pipelines, Kubernetes clusters, and monitoring tools to provide real-time insights into image security, streamline vulnerability remediation, and support regulatory compliance.

## Introduction

Containerization has revolutionized software deployment, but it introduces new security challenges. Vulnerabilities in container images can propagate rapidly across environments. Pharos addresses these challenges by providing a scalable, automated solution for scanning, reporting, and managing container image security.

## Architecture Overview

Pharos is built in Go and leverages a modular architecture:

- **Core Components**: REST API server (handles scanning), Prometheus reporter, Scheduler, and supporting modules.
- **Supported Scanners**: Integrates with Grype and Trivy for vulnerability analysis.
- **Requirements**: Relies on Redis and PostgreSQL for queueing and persistent storage.
- **Deployment**: Designed to run on Kubernetes, with Helm charts for simplified installation and management.
- **Extensibility**: Custom enrichers/mappers, plugin support, and integration with external systems.

## Key Features

- **Automated Image Scanning via REST API**: All scan requests are processed through the REST API, which orchestrates vulnerability analysis and compliance checks using Grype and Trivy.
- **Flexible Deployment**: Can be run locally, in CI/CD pipelines, or as a Kubernetes service via Helm.
- **Queue-Based Processing**: Scan requests are queued for scalable, asynchronous task management.
- **Comprehensive REST API**: Endpoints for submitting scan tasks, retrieving results, and integrating with external systems.
- **Real-Time Reporting**: Prometheus integration for metrics collection and Grafana dashboards for visualization.
- **Extensible Enrichment**: Image contexts can be enriched by plugins using Go Templating, Starlark scripts, or Yaegi Go scripting. This allows for domain-specific data augmentation, custom reporting, and advanced automation.
- **Alerting & Integration**: Scan results can trigger alerts via webhooks, such as creating Jira tickets for detected vulnerabilities.

## Workflow

1. **Image Submission**: Users or automated systems submit container images for scanning via the REST API.
2. **Task Queueing**: Scan requests are queued for processing.
3. **Scanning**: The REST API process invokes Grype or Trivy to analyze images, extract metadata, and identify vulnerabilities.
4. **Result Storage**: Scan results are stored in PostgreSQL and made available via API.
5. **Alerting & Integration**: Scan results can trigger alerts via webhooks, e.g., to create Jira tickets for detected vulnerabilities.
6. **Reporting**: Metrics are exported to Prometheus; dashboards are available in Grafana.
7. **Remediation**: Users can query vulnerabilities, track remediation status, and generate compliance reports.

## Security Model

- **Authentication**: Supports basic auth and token-based authentication for API endpoints.
- **Role-Based Access**: Integrates with Kubernetes RBAC for secure deployment.
- **Data Integrity**: Scan results are stored securely; sensitive data is protected via configuration.

## Deployment

- **Helm Chart**: Simplifies Kubernetes deployment, including all required services (Redis, PostgreSQL).
- **Grafana Dashboard**: Visualizes scan metrics and compliance status.
- **Configuration**: All parameters are configurable via CLI flags or environment variables.

## Use Cases

- **DevSecOps Automation**: Integrate Pharos into CI/CD pipelines for continuous image security.
- **Kubernetes Security**: Monitor and enforce image compliance in clusters.
- **Regulatory Compliance**: Generate reports for standards such as CIS, NIST, or custom policies.

## Future Work

- Enhanced support for additional scanners and enrichers.
- Advanced vulnerability prioritization and remediation workflows.
- Integration with external ticketing and alerting systems.

## Conclusion

Pharos provides a robust, extensible platform for container image security and compliance. Its modular design, REST API-driven workflow, and integration capabilities make it suitable for organizations seeking to automate and scale their DevSecOps practices.
