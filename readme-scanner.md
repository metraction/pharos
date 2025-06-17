# Pharos Container Image Scanner

Pharos scans container images for vulnerabilities. The Pharos scanner provides an unified interface to execute scan tasks and return results in a normalized data model, providing light or no coupling to the underlying scanner techology.

Pharos executes scans usting [Grype](https://github.com/anchore/grype) or [Trivy](https://trivy.dev/latest/), both formidable open source vulnerability scanners.

To scale and increase perfoamance, the scanner first creates the [SBOM](https://en.wikipedia.org/wiki/Software_supply_chain) of the image. The SBOM is cached and reused for subsequent scans of the scame image.
