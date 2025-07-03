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
            Digest              text,
            ImageSpec           text,
            ImageId             text,
            IndexDigest         text,
            ManifestDigest      text,
            RepoDigests         jsonb,
            ArchName            text,
            ArchOS              text,
            DistroName          text,
            DistroVersion       text,
            Size                integer,
            Tags                jsonb,
            Layers              jsonb
        )`,
		`create unique index if not exists images_digest_idx on vdb_images (Digest)`,

		// ScanMeta
		`create table if not exists vdb_scans (
            id                  integer primary key autoincrement,
            image_id            integer,
            Created             timestamptz,
            Updated             timestamptz,
            ScanDate            timestamptz,
            DbBuiltDate         timestamptz,
            Engine              string,
            EngineVersion       string,
            foreign key (image_id) references vdb_images(id)
        )`,
		`create unique index if not exists scansunique_idx on vdb_scans (image_id, Engine)`,

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
            FixVersions         text,
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
