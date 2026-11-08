package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"dapper.local/platform/internal/backup"
	"dapper.local/platform/internal/email"
	"dapper.local/platform/internal/postgres"
)

const backupWindow = 2 * time.Hour

var backupFileID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func randomHex() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// serverSecrets are the settings that unlock data saved encrypted in the database (mail password, Square tokens). A restored database is useless
// on a new server without them, so they travel inside the sealed file.
func serverSecrets() []byte {
	var lines []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "DAPPER_") && name != "DAPPER_TLS_KEY" && (strings.Contains(name, "KEY") || strings.Contains(name, "SECRET") || strings.Contains(name, "TOKEN")) {
			lines = append(lines, kv)
		}
	}
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

// sealBackup writes one complete backup file into the backup folder. The plain database dump exists only while this runs.
func sealBackup(ctx context.Context, store *postgres.Store, passphrase, name string) (string, error) {
	dir := store.BackupDirectory()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	dump := filepath.Join(dir, randomHex()+".dump")
	defer os.Remove(dump)
	if err := store.DumpDatabase(ctx, dump); err != nil {
		return "", err
	}
	final := filepath.Join(dir, name+".dapbk")
	partial := final + ".part"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer os.Remove(partial)
	err = backup.Write(f, passphrase, backup.Manifest{SchemaVersion: store.BackupSchemaVersion(), CreatedAt: time.Now().UTC()}, backup.Source{DumpPath: dump, Keys: serverSecrets(), Folders: store.BackupFolders()})
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	return final, os.Rename(partial, final)
}

// keepNewest removes all but the newest n files matching the pattern.
func keepNewest(pattern string, n int) {
	files, _ := filepath.Glob(pattern)
	slices.Sort(files) // safety files are named by time
	for len(files) > n {
		_ = os.Remove(files[0])
		files = files[1:]
	}
}

func readField(part io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(part, 1024))
	return string(b), err
}

func validPassphrase(p string) bool { return len(p) >= 12 && len(p) <= 256 }

func registerBackupRoutes(mux *http.ServeMux, store *postgres.Store, sender postgres.EmailSender) {
	base := "/api/v1/organizations/{organization}/backups"
	var running sync.Mutex // one backup or restore at a time
	allowed := func(w http.ResponseWriter, r *http.Request) bool {
		rc := http.NewResponseController(w)
		_ = rc.SetReadDeadline(time.Now().Add(backupWindow))
		_ = rc.SetWriteDeadline(time.Now().Add(backupWindow))
		if !uuid(r.PathValue("organization")) {
			reply(w, 400, map[string]string{"error": "Valid organization selector required"})
			return false
		}
		if err := store.BackupPermission(r.Context(), requestSession(r).principalID, r.PathValue("organization")); err != nil {
			failure(w, err)
			return false
		}
		if store.BackupDirectory() == "" {
			reply(w, 503, map[string]string{"error": "File storage is not set up on this server, so backups are not available."})
			return false
		}
		return true
	}

	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		if !allowed(w, r) {
			return
		}
		var in struct {
			Passphrase string `json:"passphrase"`
		}
		if !decode(w, r, &in) {
			return
		}
		if !validPassphrase(in.Passphrase) {
			reply(w, 422, map[string]string{"error": "Use a passphrase of at least 12 characters."})
			return
		}
		if !running.TryLock() {
			reply(w, 409, map[string]string{"error": "A backup or restore is already running."})
			return
		}
		defer running.Unlock()
		id := randomHex()
		// Only the newest prepared file waits on the server. Safety copies are named differently.
		stale, _ := filepath.Glob(filepath.Join(store.BackupDirectory(), "????????????????????????????????.dapbk"))
		for _, f := range stale {
			_ = os.Remove(f)
		}
		path, err := sealBackup(r.Context(), store, in.Passphrase, id)
		if err != nil {
			log.Printf("backup failed: %v", err)
			reply(w, 500, map[string]string{"error": "The backup could not be made. Nothing was changed."})
			return
		}
		var size int64
		if info, err := os.Stat(path); err == nil {
			size = info.Size()
		}
		reply(w, 201, map[string]any{"id": id, "sizeBytes": size})
	})

	mux.HandleFunc("GET "+base+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(w, r) {
			return
		}
		id := r.PathValue("id")
		if !backupFileID.MatchString(id) {
			reply(w, 400, map[string]string{"error": "Valid backup selector required"})
			return
		}
		path := filepath.Join(store.BackupDirectory(), id+".dapbk")
		f, err := os.Open(path)
		if err != nil {
			reply(w, 404, map[string]string{"error": "That backup is no longer waiting. Prepare a new one."})
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			failure(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprint(info.Size()))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="dapper-backup-%s.dapbk"`, time.Now().UTC().Format("2006-01-02")))
		w.Header().Set("Cache-Control", "no-store")
		_, err = io.Copy(w, f)
		_ = f.Close() // Windows cannot delete a file that is still open
		if err == nil {
			_ = os.Remove(path)
		}
	})

	mux.HandleFunc("POST "+base+"/restore", func(w http.ResponseWriter, r *http.Request) {
		if !allowed(w, r) {
			return
		}
		if !running.TryLock() {
			reply(w, 409, map[string]string{"error": "A backup or restore is already running."})
			return
		}
		defer running.Unlock()
		form, err := r.MultipartReader()
		if err != nil {
			reply(w, 400, map[string]string{"error": "Send the backup file with its passphrase."})
			return
		}
		var passphrase, confirm string
		var file io.Reader
		for file == nil {
			part, err := form.NextPart()
			if err != nil {
				reply(w, 400, map[string]string{"error": "Send the backup file with its passphrase."})
				return
			}
			switch part.FormName() {
			case "passphrase":
				passphrase, err = readField(part)
			case "confirm":
				confirm, err = readField(part)
			case "file":
				file = part
			}
			if err != nil {
				reply(w, 400, map[string]string{"error": "Send the backup file with its passphrase."})
				return
			}
		}
		if confirm != "RESTORE" || !validPassphrase(passphrase) {
			reply(w, 422, map[string]string{"error": "Enter the backup's passphrase and type RESTORE to confirm."})
			return
		}
		dir := store.BackupDirectory()
		if err = os.MkdirAll(dir, 0o700); err != nil {
			failure(w, err)
			return
		}
		work := filepath.Join(dir, "restore-"+randomHex())
		defer os.RemoveAll(work)
		folders := store.BackupFolders()
		names := make([]string, 0, len(folders))
		for name := range folders {
			names = append(names, name)
		}
		// Unpacking checks every chunk; nothing live is touched until the whole file has passed.
		manifest, err := backup.Extract(file, passphrase, work, names)
		if errors.Is(err, backup.ErrUnreadable) {
			reply(w, 422, map[string]string{"error": "That file could not be opened. Check the passphrase, and that it is a complete Dapper backup. Nothing was changed."})
			return
		}
		if err != nil {
			log.Printf("restore: unpacking failed: %v", err)
			reply(w, 500, map[string]string{"error": "The backup could not be unpacked. Nothing was changed."})
			return
		}
		if manifest.SchemaVersion != store.BackupSchemaVersion() {
			reply(w, 409, map[string]string{"error": fmt.Sprintf("This backup is from a different version of the system (%d, this one is %d). Nothing was changed.", manifest.SchemaVersion, store.BackupSchemaVersion())})
			return
		}
		// From here on the work must finish even if the browser goes away.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), backupWindow)
		defer cancel()
		safety := "safety-" + time.Now().UTC().Format("20060102-150405")
		if _, err = sealBackup(ctx, store, passphrase, safety); err != nil {
			log.Printf("restore: safety copy failed: %v", err)
			reply(w, 500, map[string]string{"error": "The safety copy of today's data could not be made, so nothing was changed."})
			return
		}
		keepNewest(filepath.Join(dir, "safety-*.dapbk"), 3)
		if err = store.RestoreDatabase(ctx, filepath.Join(work, backup.DumpName)); err != nil {
			log.Printf("restore: database failed: %v", err)
			reply(w, 500, map[string]string{"error": "The database could not be restored, so it was left as it was. Nothing was changed."})
			return
		}
		log.Printf("restore: database replaced from a backup made %s; safety copy %s", manifest.CreatedAt.Format(time.RFC3339), safety)
		for name, live := range folders {
			incoming := filepath.Join(work, name)
			if _, err := os.Stat(incoming); err != nil {
				continue
			}
			retired := filepath.Join(work, "old-"+name)
			if err = os.Rename(live, retired); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("restore: could not move aside %s: %v", name, err)
				reply(w, 500, map[string]string{"error": "The database was restored but the photos and documents were not. The safety copy " + safety + " is on the server."})
				return
			}
			if err = os.Rename(incoming, live); err != nil {
				_ = os.Rename(retired, live)
				log.Printf("restore: could not put back %s: %v", name, err)
				reply(w, 500, map[string]string{"error": "The database was restored but the photos and documents were not. The safety copy " + safety + " is on the server."})
				return
			}
		}
		// Settings the running service read at start must come from the restored data.
		if err = store.LoadSquareAppCredentials(ctx); err != nil {
			log.Print("restore: saved Square keys could not be reloaded")
		}
		if saved, found, err := store.SavedSMTP(ctx); err == nil && found {
			if live, ok := sender.(interface{ Set(email.Config) error }); ok {
				if err = live.Set(email.Config{Host: saved.Host, Port: saved.Port, Username: saved.Username, Password: saved.Password, FromAddress: saved.FromAddress, FromName: saved.FromName}); err != nil {
					log.Print("restore: saved email settings are not usable")
				}
			}
		}
		// A different server has different keys: encrypted settings stay unreadable until the backup's keys are put in its .env.
		keys, _ := os.ReadFile(filepath.Join(work, backup.KeysName))
		differ := string(keys) != string(serverSecrets())
		if differ {
			if err = os.WriteFile(filepath.Join(dir, "restored-keys.env"), keys, 0o600); err != nil {
				log.Print("restore: the backup's keys could not be saved")
			}
		}
		reply(w, 200, map[string]any{"restored": true, "backupCreatedAt": manifest.CreatedAt, "safetyCopy": safety, "keysDiffer": differ})
	})
}
