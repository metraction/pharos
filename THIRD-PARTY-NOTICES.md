# Third-Party Notices

This project (Pharos) includes and depends upon third-party open source software components.  
This document lists these components along with their declared module paths and versions as specified in `go.mod`.  
Licenses should be verified before distribution. Some license identifiers are best-effort and may require confirmation.

If you redistribute Pharos, you are responsible for complying with the licenses of these components.

---

## How This List Was Generated

Derived from the direct (and key indirect) dependencies declared in `go.mod`.  
For complete transitive license extraction, use a tool such as:

```
go install github.com/google/go-licenses@latest
go-licenses report ./... > THIRD_PARTY_LICENSES_FULL.txt
```

Or to collect license files:

```
go-licenses save ./... --save_path=third_party_licenses
```

---

## Legend

- Status: `direct` (explicit in go.mod), `indirect` (transitive noted in go.mod).
- License: Best-effort guess. Verify in each upstream repository.
- Homepage: Usually the Go module repository (GitHub unless noted).

---

## Core Direct Dependencies

| Module | Version | Status | Approx. License (VERIFY) | Upstream |
|--------|---------|--------|---------------------------|----------|
| github.com/CycloneDX/cyclonedx-go | v0.9.2 | direct | Apache-2.0 | https://github.com/CycloneDX/cyclonedx-go |
| github.com/acarl005/stripansi | v0.0.0-20180116102854-5a71ef0e047d | direct | MIT | https://github.com/acarl005/stripansi |
| github.com/dustin/go-humanize | v1.0.1 | direct | MIT | https://github.com/dustin/go-humanize |
| github.com/google/go-containerregistry | v0.20.5 | direct | Apache-2.0 | https://github.com/google/go-containerregistry |
| github.com/joho/godotenv | v1.5.1 | direct | MIT | https://github.com/joho/godotenv |
| github.com/klauspost/compress | v1.18.0 | direct | Apache-2.0 | https://github.com/klauspost/compress |
| github.com/kos-v/dsnparser | v1.1.0 | direct | MIT (VERIFY) | https://github.com/kos-v/dsnparser |
| github.com/package-url/packageurl-go | v0.1.3 | direct | Apache-2.0 (VERIFY) | https://github.com/package-url/packageurl-go |
| github.com/redis/go-redis/v9 | v9.10.0 | direct | BSD-2-Clause | https://github.com/redis/go-redis |
| github.com/reugn/go-streams | v0.13.0 | direct | Apache-2.0 | https://github.com/reugn/go-streams |
| github.com/rs/zerolog | v1.34.0 | direct | MIT | https://github.com/rs/zerolog |
| github.com/samber/lo | v1.50.0 | direct | MIT | https://github.com/samber/lo |
| github.com/spf13/cobra | v1.9.1 | direct | Apache-2.0 | https://github.com/spf13/cobra |
| github.com/spf13/pflag | v1.0.6 | direct | BSD-3-Clause | https://github.com/spf13/pflag |
| github.com/spf13/viper | v1.20.1 | direct | MIT | https://github.com/spf13/viper |
| github.com/stretchr/testify | v1.10.0 | direct | MIT | https://github.com/stretchr/testify |
| gorm.io/gorm | v1.30.0 | direct | MIT | https://github.com/go-gorm/gorm |
| gorm.io/driver/postgres | v1.6.0 | direct | MIT | https://github.com/go-gorm/postgres |
| github.com/Masterminds/semver/v3 | v3.3.0 | direct | MIT | https://github.com/Masterminds/semver |
| github.com/Masterminds/sprig/v3 | v3.3.0 | direct | MIT | https://github.com/Masterminds/sprig |
| github.com/alicebob/miniredis | v2.5.0+incompatible | direct | MIT | https://github.com/alicebob/miniredis |
| github.com/danielgtaylor/huma/v2 | v2.32.0 | direct | Apache-2.0 | https://github.com/danielgtaylor/huma |
| github.com/dustinkirkland/golang-petname | v0.0.0-20240428194347-eebcea082ee0 | direct | Apache-2.0 | https://github.com/dustinkirkland/golang-petname |
| github.com/go-chi/chi/v5 | v5.1.0 | direct | MIT | https://github.com/go-chi/chi |
| github.com/go-git/go-git/v5 | v5.16.2 | direct | Apache-2.0 | https://github.com/go-git/go-git |
| github.com/mattn/go-sqlite3 | v1.14.28 | direct | MIT | https://github.com/mattn/go-sqlite3 |
| github.com/metraction/handwheel | v0.0.3 | direct | (Project Internal / VERIFY) | https://github.com/metraction/handwheel |
| github.com/otiai10/copy | v1.14.1 | direct | MIT | https://github.com/otiai10/copy |
| github.com/prometheus/client_golang | v1.22.0 | direct | Apache-2.0 | https://github.com/prometheus/client_golang |
| github.com/theory/jsonpath | v0.10.0 | direct | (Artistic-2.0 or MIT? VERIFY) | https://github.com/theory/jsonpath |
| github.com/traefik/yaegi | v0.16.1 | direct | Apache-2.0 | https://github.com/traefik/yaegi |
| go.starlark.net | v0.0.0-20240123142251-f86470692795 | direct | BSD-3-Clause | https://github.com/google/starlark-go |
| gopkg.in/yaml.v2 | v2.4.0 | direct | MIT | https://gopkg.in/yaml.v2 |
| gorm.io/driver/sqlite | v1.6.0 | direct | MIT | https://github.com/go-gorm/sqlite |
| k8s.io/api | v0.33.2 | direct | Apache-2.0 | https://github.com/kubernetes/api |
| k8s.io/apimachinery | v0.33.2 | direct | Apache-2.0 | https://github.com/kubernetes/apimachinery |
| k8s.io/client-go | v0.33.2 | direct | Apache-2.0 | https://github.com/kubernetes/client-go |
| github.com/1set/starlet | v0.1.3 | direct | Apache-2.0 | https://github.com/1set/starlet |
| github.com/alicebob/miniredis/v2 | v2.35.0 | direct | MIT | https://github.com/alicebob/miniredis |
| github.com/dranikpg/gtrs | v0.6.1 | direct | MIT (VERIFY) | https://github.com/dranikpg/gtrs |
| github.com/google/uuid | v1.6.0 | direct | BSD-3-Clause | https://github.com/google/uuid |
| gopkg.in/yaml.v3 | v3.0.1 | direct | MIT | https://github.com/go-yaml/yaml |

---

## Notable Indirect (Transitive) Dependencies (Sample)

(For completeness; not exhaustive. Use automated tooling for a full list.)

| Module | Approx. License (VERIFY) |
|--------|--------------------------|
| github.com/pkg/errors | BSD-2-Clause |
| github.com/rs/xid (if pulled transitively) | MIT |
| github.com/prometheus/common | Apache-2.0 |
| github.com/prometheus/procfs | Apache-2.0 |
| golang.org/x/sync | BSD-style |
| golang.org/x/sys | BSD-style |
| golang.org/x/net | BSD-style |
| golang.org/x/crypto | BSD-style |
| golang.org/x/text | BSD-style |
| google.golang.org/protobuf | BSD-3-Clause |
| sigs.k8s.io/yaml | Apache-2.0 |
| sigs.k8s.io/structured-merge-diff/v4 | Apache-2.0 |

---

## Obtaining License Texts

Suggested command:

```
go-licenses save ./... --save_path=third_party_licenses
```

This will create a directory with each dependency’s license file(s).  
Review any with non-standard or multi-license terms.

---

## Disclaimer

- This document is provided on a best-effort basis and may be incomplete.
- Always verify licenses directly in the upstream repositories before redistribution or attribution-sensitive use.
- Some modules may embed additional code with different licenses.

---

## Suggested Automation Script (Optional)

```
#!/usr/bin/env bash
set -euo pipefail
go install github.com/google/go-licenses@latest
rm -rf third_party_licenses
go-licenses save ./... --save_path=third_party_licenses
go-licenses report ./... > THIRD_PARTY_LICENSES_FULL.txt
echo "License collection complete."
```

---

## Contact

For corrections or updates to this notice, submit a pull request or open an issue in the repository.

---