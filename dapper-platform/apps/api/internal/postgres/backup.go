package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BackupPermission allows the whole-system backup and restore: only someone who holds both top organization grants.
func (s *Store) BackupPermission(ctx context.Context, actor, org string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, action := range []string{"access.manage", "invoices.configure"} {
		ok, err := actorHasOrgCapability(ctx, tx, actor, org, action)
		if err != nil {
			return err
		}
		if !ok {
			return ErrForbidden
		}
	}
	return nil
}

// BackupSchemaVersion is the newest migration this build knows.
func (s *Store) BackupSchemaVersion() int {
	all := migrations()
	return all[len(all)-1].version
}

// BackupFolders names the file folders a backup carries, and BackupDirectory where backup files wait. Both sit together so a restore can swap
// folders with a rename. Empty when uploads are not configured.
func (s *Store) BackupFolders() map[string]string {
	if s.uploadDir == "" {
		return nil
	}
	folders := map[string]string{"uploads": s.uploadDir}
	if s.sopDir != "" {
		folders["sops"] = s.sopDir
	}
	return folders
}

func (s *Store) BackupDirectory() string {
	if s.uploadDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(s.uploadDir), "backups")
}

// pgEnv passes the connection through the environment, so the password never appears in a process list.
func (s *Store) pgEnv() []string {
	c := s.pool.Config().ConnConfig
	sslmode := "disable"
	if c.TLSConfig != nil {
		sslmode = "require"
	}
	return append(os.Environ(), "PGHOST="+c.Host, fmt.Sprintf("PGPORT=%d", c.Port), "PGUSER="+c.User, "PGPASSWORD="+c.Password, "PGDATABASE="+c.Database, "PGSSLMODE="+sslmode)
}

func (s *Store) runPG(ctx context.Context, tool string, args ...string) error {
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = s.pgEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return errors.New(tool + " failed: " + err.Error() + ": " + strings.TrimSpace(stderr.String()))
	}
	return nil
}

// DumpDatabase writes the whole database to path.
func (s *Store) DumpDatabase(ctx context.Context, path string) error {
	return s.runPG(ctx, "pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--file="+path)
}

// RestoreDatabase replaces the whole database with a dump. It is one transaction: on any error the live data stays exactly as it was.
func (s *Store) RestoreDatabase(ctx context.Context, path string) error {
	return s.runPG(ctx, "pg_restore", "--clean", "--if-exists", "--no-owner", "--no-privileges", "--single-transaction", "--exit-on-error", "--dbname="+s.pool.Config().ConnConfig.Database, path)
}
