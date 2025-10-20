package grype

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/metraction/pharos/internal/utils"
	cpy "github.com/otiai10/copy"
	"github.com/samber/lo"
)

// helper

// return human readable vuln db status
func dbNiceState(updateRequired bool) string {
	return lo.Ternary(updateRequired, "expired", "up-to-date")
}

func TranslateMessage(msg string) string {
	// translate as original messages that are missleading is missleading
	msg = strings.Replace(msg, "No vulnerability database update available", "OK, no update required", 1)
	msg = strings.TrimSpace(msg)
	return msg
}

// grype update check (from: grype db check -o json)
type GrypeDbCheck struct {
	UpdateAvailable bool `json:"updateAvailable"`
}

// grype version check (from: grype version -o json)
type GrypeVersion struct {
	Application  string    `json:"application"`
	BuildDate    time.Time `json:"buildDate"`
	Platform     string    `json:"platform"`
	GrypeVersion string    `json:"version"`
	SyftVersion  string    `json:"syftVersion"`
	//SupportedDbSchema int       `json:"supportedDbSchema"`
}

// grype local database status (from: grype db status -o json)
type GrypeDbStatus struct {
	SchemaVersion string    `json:"schemaVersion"`
	From          string    `json:"from"`
	Built         time.Time `json:"built"`
	Path          string    `json:"path"`
	Valid         bool      `json:"valid"`
	Error         string    `json:"error"`
}

// run grype executable, parse result from json into T
func GrypeExeOutput[T any](cmd *exec.Cmd) (T, error) {

	var result T
	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// execute command
	err1 := cmd.Run()
	err2 := json.Unmarshal(stdout.Bytes(), &result)

	if err1 != nil || err2 != nil {
		msg := TranslateMessage(stderr.String())
		return result, fmt.Errorf("%s", utils.NoColorCodes(msg))
	}
	return result, nil
}

// check grype binary version
func GetScannerVersion(scannerBin string) (string, error) {

	cmd := exec.Command(scannerBin, "version", "-o", "json")
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")

	result, err := GrypeExeOutput[GrypeVersion](cmd)
	if err != nil {
		return "", err
	}
	return result.GrypeVersion, nil
}

// check grype local database status, update DbState
func GetDatabaseStatus(scannerBin string, targetDir string) (string, time.Time, error) {

	cmd := exec.Command(scannerBin, "db", "status", "-o", "json")
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+targetDir)

	result, err := GrypeExeOutput[GrypeDbStatus](cmd)
	if err != nil {
		return "", time.Time{}, err
	}
	return result.SchemaVersion, result.Built, nil
}

// check if local db in targetDir requires an update
func GrypeUpdateRequired(scannerBin, targetDir string) bool {

	cmd := exec.Command(scannerBin, "db", "check", "-o", "json")
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+targetDir)

	grypeCheck, err := GrypeExeOutput[GrypeDbCheck](cmd)
	if err != nil {
		return true
	}
	return grypeCheck.UpdateAvailable
}

// check if update is required, if so download to targetDir
func GetGrypeUpdate(scannerBin, targetDir string) error {

	// do update
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(scannerBin, "db", "update")
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+targetDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	fmt.Println("grype update output:", utils.NoColorCodes(stdout.String()))
	if err != nil {
		return fmt.Errorf("grype update %s [%s]", utils.NoColorCodes(stderr.String()), targetDir)
	}
	return nil
}

// test scanner with actual scan of sample test sbom
func GrypeTestScan(scannerBin, targetDir string) error {

	sampleSbom := []byte(`{"artifacts":[],"artifactRelationships":[],"source":{"id":"509fea71c91a0fbb0e4034193e894571ca5c9270068538e41a155ad5fbcad0a8","name":"go.mod.test","version":"sha256:34228366858d96d4c070b3e540bdf7653962f504dbec3648b90e506fba321325","type":"file","metadata":{"path":"./go.mod.test","digests":[{"algorithm":"sha256","value":"34228366858d96d4c070b3e540bdf7653962f504dbec3648b90e506fba321325"}],"mimeType":"text/plain"}},"distro":{},"descriptor":{"name":"syft","version":"1.26.1","configuration":{"catalogers":{"requested":{"default":["directory","file"]},"used":["alpm-db-cataloger","apk-db-cataloger","binary-classifier-cataloger","cargo-auditable-binary-cataloger","cocoapods-cataloger","conan-cataloger","dart-pubspec-cataloger","dart-pubspec-lock-cataloger","deb-archive-cataloger","dotnet-deps-binary-cataloger","dotnet-packages-lock-cataloger","dpkg-db-cataloger","elf-binary-package-cataloger","elixir-mix-lock-cataloger","erlang-otp-application-cataloger","erlang-rebar-lock-cataloger","file-content-cataloger","file-digest-cataloger","file-executable-cataloger","file-metadata-cataloger","github-action-workflow-usage-cataloger","github-actions-usage-cataloger","go-module-binary-cataloger","go-module-file-cataloger","graalvm-native-image-cataloger","haskell-cataloger","homebrew-cataloger","java-archive-cataloger","java-gradle-lockfile-cataloger","java-jvm-cataloger","java-pom-cataloger","javascript-lock-cataloger","linux-kernel-cataloger","lua-rock-cataloger","nix-cataloger","opam-cataloger","pe-binary-package-cataloger","php-composer-lock-cataloger","php-interpreter-cataloger","php-pear-serialized-cataloger","portage-cataloger","python-installed-package-cataloger","python-package-cataloger","r-package-cataloger","rpm-archive-cataloger","rpm-db-cataloger","ruby-gemfile-cataloger","ruby-gemspec-cataloger","rust-cargo-lock-cataloger","swift-package-manager-cataloger","swipl-pack-cataloger","terraform-lock-cataloger","wordpress-plugins-cataloger"]},"data-generation":{"generate-cpes":true},"files":{"content":{"globs":null,"skip-files-above-size":0},"hashers":["sha-1","sha-256"],"selection":"owned-by-package"},"licenses":{"coverage":75,"include-content":"none"},"packages":{"binary":["python-binary","python-binary-lib","pypy-binary-lib","go-binary","julia-binary","helm","redis-binary","java-binary-openjdk","java-binary-ibm","java-binary-oracle","java-binary-graalvm","java-binary-jdk","nodejs-binary","go-binary-hint","busybox-binary","util-linux-binary","haproxy-binary","perl-binary","php-composer-binary","httpd-binary","memcached-binary","traefik-binary","arangodb-binary","postgresql-binary","mysql-binary","mysql-binary","mysql-binary","xtrabackup-binary","mariadb-binary","rust-standard-library-linux","rust-standard-library-macos","ruby-binary","erlang-binary","erlang-alpine-binary","erlang-library","swipl-binary","dart-binary","haskell-ghc-binary","haskell-cabal-binary","haskell-stack-binary","consul-binary","nginx-binary","bash-binary","openssl-binary","gcc-binary","fluent-bit-binary","wordpress-cli-binary","curl-binary","lighttpd-binary","proftpd-binary","zstd-binary","xz-binary","gzip-binary","sqlcipher-binary","jq-binary","chrome-binary"],"dotnet":{"dep-packages-must-claim-dll":true,"dep-packages-must-have-dll":false,"propagate-dll-claims-to-parents":true,"relax-dll-claims-when-bundling-detected":true},"golang":{"local-mod-cache-dir":"/Users/sam/go/pkg/mod","local-vendor-dir":"","main-module-version":{"from-build-settings":true,"from-contents":false,"from-ld-flags":true},"proxies":["https://proxy.golang.org","direct"],"search-local-mod-cache-licenses":false,"search-local-vendor-licenses":false,"search-remote-licenses":false},"java-archive":{"include-indexed-archives":true,"include-unindexed-archives":false,"maven-base-url":"https://repo1.maven.org/maven2","maven-localrepository-dir":"/Users/sam/.m2/repository","max-parent-recursive-depth":0,"resolve-transitive-dependencies":false,"use-maven-localrepository":false,"use-network":false},"javascript":{"include-dev-dependencies":false,"npm-base-url":"https://registry.npmjs.org","search-remote-licenses":false},"linux-kernel":{"catalog-modules":true},"nix":{"capture-owned-files":false},"python":{"guess-unpinned-requirements":false}},"relationships":{"exclude-binary-packages-with-file-ownership-overlap":true,"package-file-ownership":true,"package-file-ownership-overlap":true},"search":{"scope":"squashed"}}},"schema":{"version":"16.0.34","url":"https://raw.githubusercontent.com/anchore/syft/main/schema/json/schema-16.0.34.json"}}`)

	var stdout, stderr bytes.Buffer

	cmd := exec.Command(scannerBin, "-o", "json")
	cmd.Stdin = bytes.NewReader(sampleSbom)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// check https://github.com/anchore/grype
	cmd.Env = append(cmd.Env, "GRYPE_CHECK_FOR_APP_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_DB_AUTO_UPDATE=false")
	cmd.Env = append(cmd.Env, "GRYPE_DB_CACHE_DIR="+targetDir)

	err := cmd.Run()
	//data := stdout.Bytes() // results as []byte
	//fmt.Println("result", len(data))
	return err

}

// deploy staged update from sourceDir into targetDir
func DeployStagedUpdate(sourceDir, targetDir string) error {
	// validate
	if !utils.DirExists(sourceDir) {
		return fmt.Errorf("deploy staged update: source not found: %v", sourceDir)
	}
	// delete target, then copy source to target
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("deploy staged update: source not found: %v", sourceDir)
	}
	// copy
	if err := cpy.Copy(sourceDir, targetDir); err != nil {
		return fmt.Errorf("deploy staged update: cannot copy to %v", targetDir)
	}
	return nil
}
