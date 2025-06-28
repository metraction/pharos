package localdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/marcboeker/go-duckdb"
	_ "github.com/marcboeker/go-duckdb"
	"github.com/metraction/pharos/pkg/model"
	"github.com/rs/zerolog"
)

func duckStrList(input []string) string {

	quoted := make([]string, len(input))
	for i, s := range input {
		quoted[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(s, "'", "''"))
	}
	return fmt.Sprintf("[%s]", strings.Join(quoted, ","))
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
	var connector *duckdb.Connector
	if connector, err = duckdb.NewConnector(rx.Endpoint, InitPharosDb); err != nil {
		return err
	}
	defer connector.Close()
	rx.db = sql.OpenDB(connector)

	// if rx.conn, err = connector.Connect(ctx); err != nil {
	// 	return err
	// }
	return rx.db.Ping()
}
func (rx *PharosLocalDb) Close() {
	if rx.db != nil {
		rx.db.Close()
	}
}

// add complete scan result
func (rx *PharosLocalDb) AddScanResult(ctx context.Context, result model.PharosScanResult) (uint64, error) {

	// add image
	var err error
	var imgid uint64
	if imgid, err = rx.AddImage(ctx, result.Image); err != nil {
		return 0, err
	}
	if err := rx.AddVulns(ctx, result.Vulnerabilities); err != nil {
		return 0, err
	}

	return imgid, nil
}

func (rx *PharosLocalDb) AddImage(ctx context.Context, image model.PharosImageMeta) (uint64, error) {

	sqlcmd := `
		insert into vdb.images (
			Created, Updated,
			ImageSpec, ImageId, IndexDigest, ManifestDigest, RepoDigests,
			ArchName, ArchOS, DistroName, DistroVersion, Size,
			Tags, Layers
		) values (
			?, ?,
			?, ?, ?, ?, ?,
			?, ?, ?, ?, ?,
			?, ?
		)
		on conflict (ManifestDigest) do update set
			updated = excluded.Updated
		returning id
	`
	var err error
	var id uint64
	err = rx.db.QueryRow(sqlcmd,
		time.Now().UTC(), time.Now().UTC(),
		image.ImageSpec, image.ImageId, image.IndexDigest, image.ManifestDigest,
		duckStrList(image.RepoDigests),
		image.ArchName, image.ArchOS, image.DistroName, image.DistroVersion, image.Size,
		duckStrList(image.Tags), duckStrList(image.Layers)).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, err
}

// add vulnerabilities
func (rx *PharosLocalDb) AddVulns(ctx context.Context, vulns []model.PharosVulnerability) error {

	sqlcmd := `
		insert into vdb.vulns (
			Created, Updated,
			AdvId, AdvSource, AdvAliases,
			CreateDate, PubDate, ModDate, KevDate,
			Severity, CvssVectors, CvssBase, RiskScore,
			Cpes, Cwes, Refs,
			Ransomware, Description
		) values (
			?, ?,
			?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?
		)
		on conflict (AdvId,AdvSource) do update set
			updated = excluded.Updated
		returning id
	`
	var err error

	for _, vuln := range vulns {
		_, err = rx.db.Exec(sqlcmd,
			time.Now().UTC(), time.Now().UTC(),
			vuln.AdvId, vuln.AdvSource, vuln.AdvAliases,
			vuln.CreateDate, vuln.PubDate, vuln.ModDate, vuln.KevDate,
			vuln.Severity, duckStrList(vuln.CvssVectors), vuln.CvssBase, vuln.RiskScoce,
			duckStrList(vuln.Cpes), duckStrList(vuln.Cwes), duckStrList(vuln.References),
			vuln.RansomwareUsed, vuln.Description)
		if err != nil {
			return err
		}
	}
	return nil
}
