package dbl

// Pharos local db for testing and results validation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/metraction/pharos/internal/utils"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

// convert []string to json for sqlite jsonb
func dbStrList(input []string) string {
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "[]"
	}
	return string(jsonBytes)
}

type PharosLocalDb struct {
	Endpoint string

	//conn   driver.Conn
	db     *sql.DB
	logger *zerolog.Logger
}

// create new db instance
func NewPharosLocalDb(endpoint string, logger *zerolog.Logger) (*PharosLocalDb, error) {

	result := PharosLocalDb{
		Endpoint: endpoint,
		logger:   logger,
	}
	return &result, nil
}
func (rx *PharosLocalDb) ServiceName() string {
	return "pharos-ldb"
}

// connect and initialize if required
func (rx *PharosLocalDb) Connect(ctx context.Context) error {
	var err error

	if rx.db, err = sql.Open("sqlite3", rx.Endpoint); err != nil {
		return err
	}
	if err = rx.db.Ping(); err != nil {
		return err
	}
	// indepontent schema creation
	if err = InitPharosDbSchema(rx.db); err != nil {
		return err
	}
	return nil
}

func (rx *PharosLocalDb) Close() {
	if rx.db != nil {
		rx.db.Close()
	}
}

// add scan result with image meta, findings, and vulns into db
func (rx *PharosLocalDb) AddScanResult(ctx context.Context, result model.PharosScanResult) (uint64, error) {

	var err error
	var image_id uint64

	task := result.ScanTask
	// TODO: Use provided task.ContextRootKey
	contextKey := strings.ToLower(utils.PropOr(task.Context, "cluster", "nope") + "/" + utils.PropOr(task.Context, "namespace", "nope"))

	// add image
	if image_id, err = rx.AddImage(ctx, result.Image); err != nil {
		return 0, fmt.Errorf("addImage: %w", err)
	}
	// add rootcontext & context
	if _, err = rx.AddContext(ctx, image_id, "scan", contextKey, task.Context); err != nil {
		return 0, fmt.Errorf("addContext: %w", err)
	}

	// add vulnerabilities
	if err := rx.AddVulns(ctx, result.Vulnerabilities); err != nil {
		return 0, fmt.Errorf("addVulns: %w", err)
	}
	// TODO: add packages
	// add findings
	if err := rx.AddFindings(ctx, image_id, result.Findings); err != nil {
		return 0, fmt.Errorf("addFindings: %w", err)
	}

	return image_id, nil
}

func (rx *PharosLocalDb) AddImage(ctx context.Context, image model.PharosImageMeta) (uint64, error) {

	sqlcmd := `
		insert into vdb_images (
			Created, Updated, Digest, ImageSpec, ImageId,
			IndexDigest, ManifestDigest, RepoDigests, ArchName, ArchOS,
			DistroName, DistroVersion, Size, Tags, Layers
		) values (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?
		)
		on conflict (Digest) do update set
			updated = excluded.Updated
		returning id
	`
	var err error
	var id uint64

	now := time.Now().UTC()

	err = rx.db.QueryRow(sqlcmd,
		now, now, image.ManifestDigest, image.ImageSpec, image.ImageId,
		image.IndexDigest, image.ManifestDigest, dbStrList(image.RepoDigests), image.ArchName, image.ArchOS,
		image.DistroName, image.DistroVersion, image.Size, dbStrList(image.Tags), dbStrList(image.Layers)).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, err
}

// add context
func (rx *PharosLocalDb) AddContext(ctx context.Context, image_id uint64, source, contextKey string, xcontext map[string]any) (uint64, error) {

	var err error
	var id uint64
	var root_id uint64

	var xdata []byte
	if xdata, err = json.Marshal(xcontext); err != nil {
		return 0, err
	}
	// TODO: Simulate for now
	now := time.Now().UTC()
	expired := time.Now().Add(5 * time.Minute)

	sqlcmd := `
		insert into vdb_contextroot (
			image_id, Created, Updated, Expired, ContextKey
		) values (
			?, ?, ?, ?, ?
		)
		on conflict (image_id, ContextKey) do update set
			Updated = excluded.Updated,
			Expired = excluded.Expired
		returning id
	`
	err = rx.db.QueryRow(sqlcmd, image_id, now, now, expired, contextKey).Scan(&root_id)
	if err != nil {
		return 0, fmt.Errorf("vdb_contextroot: %w", err)
	}

	sqlcmd = `
		insert into vdb_context (
			root_id, Created, Updated, Source, Context
		) values (
			?, ?, ?, ?, ?
		)
		on conflict (root_id, Source) do update set
			Updated = excluded.Updated,
			Context = excluded.Context
		returning id
	`
	err = rx.db.QueryRow(sqlcmd, root_id, now, now, source, string(xdata)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("vdb_context: %w", err)
	}

	return root_id, err
}

// add vulnerabilities
func (rx *PharosLocalDb) AddVulns(ctx context.Context, vulns []model.PharosVulnerability) error {

	now := time.Now().UTC()
	sqlcmd := `
		insert into vdb_vulns (
			Created, Updated, AdvId, AdvSource, AdvAliases,
			CreateDate, PubDate, ModDate, KevDate, Severity,
			CvssVectors, CvssBase, RiskScore, Cpes, Cwes,
			Refs, Ransomware, Description
		) values (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?
		)
		on conflict (AdvId,AdvSource) do update set
			updated = excluded.Updated
		returning id
	`
	var err error

	for _, vuln := range vulns {
		_, err = rx.db.Exec(sqlcmd,
			now, now, vuln.AdvId, vuln.AdvSource, vuln.AdvAliases,
			vuln.CreateDate, vuln.PubDate, vuln.ModDate, vuln.KevDate, vuln.Severity,
			dbStrList(vuln.CvssVectors), vuln.CvssBase, vuln.RiskScoce, dbStrList(vuln.Cpes), dbStrList(vuln.Cwes),
			dbStrList(vuln.References), vuln.RansomwareUsed, vuln.Description)
		if err != nil {
			return err
		}
	}
	return nil
}

// add findings
func (rx *PharosLocalDb) AddFindings(ctx context.Context, image_id uint64, findings []model.PharosScanFinding) error {

	now := time.Now().UTC()
	sqlcmd := `
		insert into vdb_findings (
			i
			Created, Updated, AdvId, AdvSource, AdvAliases,
			CreateDate, PubDate, ModDate, KevDate, Severity,
			CvssVectors, CvssBase, RiskScore, Cpes, Cwes,
			Refs, Ransomware, Description
		) values (
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?
		)
		on conflict (AdvId,AdvSource) do update set
			updated = excluded.Updated
		returning id
	`
	var err error

	for _, vuln := range vulns {
		_, err = rx.db.Exec(sqlcmd,
			now, now, vuln.AdvId, vuln.AdvSource, vuln.AdvAliases,
			vuln.CreateDate, vuln.PubDate, vuln.ModDate, vuln.KevDate, vuln.Severity,
			dbStrList(vuln.CvssVectors), vuln.CvssBase, vuln.RiskScoce, dbStrList(vuln.Cpes), dbStrList(vuln.Cwes),
			dbStrList(vuln.References), vuln.RansomwareUsed, vuln.Description)
		if err != nil {
			return err
		}
	}
	return nil
}
