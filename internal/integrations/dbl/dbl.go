package dbl

// Pharos local db for testing and results validation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

// safely close connection
func (rx *PharosLocalDb) Close() {
	if rx.db != nil {
		rx.db.Close()
	}
}

// add scan result with image meta, findings, and vulns into db
func (rx *PharosLocalDb) AddScanResult(ctx context.Context, result model.PharosScanResult) (uint64, error) {

	var err error
	var scan_id uint64
	var image_id uint64

	task := result.ScanTask

	// add image
	if image_id, err = rx.AddImage(ctx, result.Image); err != nil {
		return 0, fmt.Errorf("addImage: %w", err)
	}
	// TODO: add scan, use new ScanEngine onject
	if scan_id, err = rx.AddScanMeta(ctx, image_id, result.ScanMeta); err != nil {
		return 0, fmt.Errorf("addScan: %w", err)
	}
	// add rootcontext & context
	if _, err = rx.AddContext(ctx, image_id, "scan", task.ContextRootKey, task.Context); err != nil {
		return 0, fmt.Errorf("addContext: %w", err)
	}
	// add packages
	if err := rx.AddPackages(ctx, image_id, result.Packages); err != nil {
		return 0, fmt.Errorf("addPackages: %w", err)
	}
	// add vulnerabilities
	if err := rx.AddVulns(ctx, result.Vulnerabilities); err != nil {
		return 0, fmt.Errorf("addVulns: %w", err)
	}
	// add findings
	if err := rx.AddFindings(ctx, scan_id, result.Findings); err != nil {
		return 0, fmt.Errorf("addFindings: %w", err)
	}

	return image_id, nil
}

// add image
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
	now := time.Now().UTC()

	var err error
	var id uint64

	err = rx.db.QueryRow(sqlcmd,
		now, now, image.ManifestDigest, image.ImageSpec, image.ImageId,
		image.IndexDigest, image.ManifestDigest, dbStrList(image.RepoDigests), image.ArchName, image.ArchOS,
		image.DistroName, image.DistroVersion, image.Size, dbStrList(image.Tags), dbStrList(image.Layers)).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, err
}

// add scan
func (rx *PharosLocalDb) AddScanMeta(ctx context.Context, imageId uint64, scan model.PharosScanMeta) (uint64, error) {

	sqlcmd := `
		insert into vdb_scans (
			image_id, Created, Updated, ScanDate, DbBuiltDate,
			Elapsed, Engine, EngineVersion
		) values (
			?, ?, ?, ?, ?,
			?, ?, ?
		)
		on conflict (image_id, Engine) do update set
			Updated = excluded.Updated,
			ScanDate = excluded.ScanDate,
			DbBuiltDate = excluded.DbBuiltDate,
			Elapsed = excluded.Elapsed
		returning id
	`
	now := time.Now().UTC()

	var err error
	var id uint64

	err = rx.db.QueryRow(sqlcmd, imageId, now, now, scan.ScanDate, scan.DbBuiltDate, scan.ScanElapsed.Seconds(), scan.Engine, scan.EngineVersion).Scan(&id)
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
	now := time.Now().UTC()
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
func (rx *PharosLocalDb) AddFindings(ctx context.Context, scan_id uint64, findings []model.PharosScanFinding) error {
	sqlcmd := `
		insert into vdb_findings (
			scan_id, vuln_id,
			Created, Updated, DueDate,
			Severity, FixState, FixVersions, FoundIn
		)
		select
			? 				as scan_id,
			v.id			as vuln_id,
			datetime('now') as Created,
			datetime('now') as Updated,
			?				as DueDate,
			?				as Severity,
			?				as FixState,
			?				as FixVersions,
			?				as FoundIn
		from vdb_vulns v
		where
			v.AdvId=? and v.AdvSource=?
		on conflict (scan_id, vuln_id) do update set
			Updated = excluded.Updated,
			DueDate = excluded.DueDate
		returning *
	`
	var err error

	for _, finding := range findings {
		_, err = rx.db.Exec(sqlcmd, scan_id, finding.DueDate, finding.Severity,
			finding.FixState, dbStrList(finding.FixVersions), dbStrList(finding.FoundIn),
			finding.AdvId, finding.AdvSource)
		if err != nil {
			return err
		}
	}
	return nil
}

// add packages
func (rx *PharosLocalDb) AddPackages(ctx context.Context, image_id uint64, packages []model.PharosPackage) error {

	sqlcmd1 := `
		insert into vdb_packages (
			Key, Name, Version, Type, Purl, Cpes
		) values (
			?, ?, ?, ?, ?, ?
		)
		on conflict (Key) do update set
			Cpes = excluded.Cpes
		returning id
	`
	sqlcmd2 := `
		insert into vdb_image2package (
			image_id, pack_id
		) values (
			?, ?
		)
		on conflict (image_id, pack_id) do nothing
	`
	var err error
	var pack_id uint64
	for _, pack := range packages {
		if pack.Key == "" {
			pack.Key = pack.Purl
		}
		err = rx.db.QueryRow(sqlcmd1, pack.Key, pack.Name, pack.Version, pack.Type, pack.Purl, dbStrList(pack.Cpes)).Scan(&pack_id)
		if err != nil {
			return err
		}
		_, err = rx.db.Exec(sqlcmd2, image_id, pack_id)
		if err != nil {
			return err
		}

	}
	return nil

}
