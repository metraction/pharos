package localdb

import (
	"context"
	"database/sql/driver"
)

// this is obsolete for pgsql

// initialize local pharos test db
func InitPharosDb(execer driver.ExecerContext) error {

	var err error
	cmds := []string{
		`set schema=main`,
		`set search_path=main`,

		// Schema
		`create schema if not exists vdb`,

		// ScanTask
		`create sequence if not exists scantasks_id start 1`,
		`create table if not exists vdb.scantasks (
            id              integer primary key default nextval('scantasks_id'),
            JobId           text,
            Image           text,
            Created         timestamptz,
            Updated         timestamptz,
            Status          text,
            Error           text,
        )`,

		// Images
		`create sequence if not exists images_id start 1`,
		`create table if not exists vdb.images (
            id                  uint64 primary key default nextval('images_id'),
            Created             timestamptz,
            Updated             timestamptz,
            ImageSpec           text,
            ImageId             text,
            IndexDigest         text,
            ManifestDigest      text,
            RepoDigests         text[],
            ArchName            text,
            ArchOS              text,
            DistroName          text,
            DistroVersion       text,
            Size                uint64,
            Tags                text[],
            Layers              text[],
        )`,
		`create unique index if not exists images_manifestdigest_idx on vdb.images (ManifestDigest)`,

		// Vulnerabilities
		`create sequence if not exists vulns_id start 1`,
		`create table if not exists vdb.vulns (
            id                  uint64 primary key default nextval('vulns_id'),
            Created             timestamptz,
            Updated             timestamptz,
            AdvId               text,
            AdvSource           text,
            AdvAliases          text,
            CreateDate          timestamptz,
            PubDate             timestamptz,
            ModDate             timestamptz,
            KevDate             timestamptz,
            Severity            text,
            CvssVectors         text[],
            CvssBase            double,
            RiskScore           double,
            Cpes                text[],
            Cwes                text[],
            Refs                text[],
            Ransomware          text,
            Description         text,
        )`,
		`create unique index if not exists vulns_advisory_idx on vdb.vulns (AdvId,AdvSource)`,

		// BaseContext
		`create sequence if not exists contexta_id start 1`,
		`create table if not exists vdb.contexta (
            id                  uint64 primary key default nextval('contexta_id'),
            image_id            uint64,
            Created             timestamptz,
            Updated             timestamptz,
            Expired             timestamptz,    
            ContextKey          text,
            Context             json,
            foreign key (image_id) references vdb.images(id)
        )`,
		`create unique index if not exists contexta_key_idx on vdb.contexta (ContextKey)`,
	}
	for _, sql := range cmds {
		_, err = execer.ExecContext(context.Background(), sql, nil)
		if err != nil {
			return err
		}
	}
	return nil
}
