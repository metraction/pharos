package dbl

import (
	"database/sql"
	"fmt"
)

// this is obsolete for pgsql

// initialize local pharos test db
func InitPharosDbSchema(db *sql.DB) error {

	cmds := []string{

		// Pragmas
		`pragma foreign_keys = on`, // enforce foreign keys

		// ScanTasks
		`create table if not exists vdb_scantasks (
            id              integer primary key autoincrement,
            JobId           text,
            Image           text,
            Created         timestamptz,
            Updated         timestamptz,
            Status          text,
            Error           text
        )`,

		// Images
		// TODO: Digest as unique key
		`create table if not exists vdb_images (
            id                  integer primary key autoincrement,
            Created             timestamptz,
            Updated             timestamptz,
            ImageKey            text,
            ImageId             text,
            ImageSpec           text,
            ArchName            text,
            ArchOS              text,
            DistroName          text,
            DistroVersion       text,
            Size                integer,
            Tags                jsonb,
            IndexDigest         text,
            ManifestDigest      text,
            RepoDigests         jsonb,
            Layers              jsonb
        )`,
		`create unique index if not exists images_uniqu_idx on vdb_images (ImageId)`,

		// ScanMeta
		`create table if not exists vdb_scans (
            id                  integer primary key autoincrement,
            image_id            integer,
            Created             timestamptz,
            Updated             timestamptz,
            ScanDate            timestamptz,
            DbBuiltDate         timestamptz,
            Elapsed             double,     -- scan time in sec
            Engine              text,
            EngineVersion       text,
            foreign key (image_id) references vdb_images(id)
        )`,
		`create unique index if not exists scans_unique_idx on vdb_scans (image_id, Engine)`,

		// Package
		`create table if not exists vdb_packages (
            id                  integer primary key autoincrement,
            Key                 text,
            Name                text,
            Version             text,
            Type                text,
            Purl                text,
            Cpes                jsonb
        )`,
		`create unique index if not exists packages_unique_key on vdb_packages (Key)`,

		// PackageMap
		`create table if not exists vdb_image2package (
            id                  integer primary key autoincrement,
            image_id            integer,
            pack_id             integer,
            foreign key (image_id) references vdb_images(id),
            foreign key (pack_id) references vdb_packages(id)
        )`,
		`create unique index if not exists image2packages_uniq_idx on vdb_image2package (image_id, pack_id)`,

		// Vulnerabilities
		`create table if not exists vdb_vulns (
            id                  integer primary key autoincrement,
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
            CvssVectors         jsonb,
            CvssBase            double,
            RiskScore           double,
            Cpes                jsonb,
            Cwes                jsonb,
            Refs                jsonb,
            Ransomware          text,
            Description         text
        )`,
		`create unique index if not exists vulns_advisory_idx on vdb_vulns (AdvId, AdvSource)`,

		// Findings
		`create table if not exists vdb_findings (
            id                  integer primary key autoincrement,
            scan_id             integer,
            vuln_id             integer,
            Created             timestamptz,
            Updated             timestamptz,
            DueDate             timestamptz,
            Severity            text,
            FixState            text,
            FixVersions         jsonb,
            FoundIn             jsonb,
            foreign key (scan_id) references vdb_scans(id),
            foreign key (vuln_id) references vdb_vulns(id)
        )`,
		`create unique index if not exists findings_uniqe_idx on vdb_findings (scan_id, vuln_id)`,

		// ContextRoot
		`create table if not exists vdb_contextroot (
            id                  integer primary key autoincrement,
            image_id            integer,
            Created             timestamptz,
            Updated             timestamptz,
            Expired             timestamptz,
            ContextKey          text,
            foreign key (image_id) references vdb_images(id)
        )`,
		`create unique index if not exists contextroot_key_idx on vdb_contextroot (image_id, ContextKey)`,

		// Context
		`create table if not exists vdb_context (
            id                  integer primary key autoincrement,
            root_id             integer,
            Created             timestamptz,
            Updated             timestamptz,
            Source              test,
            Context             jsonb,
            foreign key (root_id) references vdb_contextroot(id)
        )`,
		`create unique index if not exists context_rootkey_idx on vdb_context (root_id, Source)`,
	}

	for _, sqlcmd := range cmds {
		if _, err := db.Exec(sqlcmd); err != nil {
			fmt.Println("sqlcmd", sqlcmd)
			return err
		}
	}
	return nil
}
