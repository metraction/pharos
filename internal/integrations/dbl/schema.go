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
            image_id            integer,
            vuln_id             integer
            Created             timestamptz,
            Updated             timestamptz,
            AdvId               text,
            AdvSource           text,
            ScanDate            timestamptz,
            DueDate             timestamptz,
            Severity            text,
            FixState            text,
            FixVersion          text,
            FoundIn             jsonb,
            foreign key (image_id) references vdb_images(id),
            foreign key (vuln_id) references vdb_vulns(id)
        )`,
		`create unique index if not exists findings_advs_idx on vdb_findings (image_id, vuln_id)`,

		// BaseContext
		`create table if not exists vdb_contexta (
            id                  integer primary key autoincrement,
            image_id            integer,
            Created             timestamptz,
            Updated             timestamptz,
            Expired             timestamptz,    
            ContextKey          text,
            Context             jsonb,
            foreign key (image_id) references vdb_images(id)
        )`,
		`create unique index if not exists contexta_key_idx on vdb_contexta (ContextKey)`,
	}

	for _, sqlcmd := range cmds {
		if _, err := db.Exec(sqlcmd); err != nil {
			fmt.Println("sqlcmd", sqlcmd)
			return err
		}
	}
	return nil
}
