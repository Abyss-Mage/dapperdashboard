package backup

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var folders = []string{"uploads", "sops"}

func sample(t *testing.T) (Source, []byte) {
	t.Helper()
	dir := t.TempDir()
	dump := filepath.Join(dir, "d.dump")
	big := bytes.Repeat([]byte("synthetic database bytes "), 100000) // spans several chunks
	if err := os.WriteFile(dump, big, 0o600); err != nil {
		t.Fatal(err)
	}
	uploads := filepath.Join(dir, "up", "2026")
	if err := os.MkdirAll(uploads, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "car.jpg"), []byte("synthetic photo"), 0o600); err != nil {
		t.Fatal(err)
	}
	return Source{DumpPath: dump, Keys: []byte("DAPPER_MFA_KEY=synthetic\n"), Folders: map[string]string{"uploads": filepath.Join(dir, "up"), "sops": filepath.Join(dir, "missing")}}, big
}

func seal(t *testing.T, src Source) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := Write(&out, "correct horse battery", Manifest{SchemaVersion: 97, CreatedAt: time.Now()}, src); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestRoundTrip(t *testing.T) {
	src, big := sample(t)
	file := seal(t, src)
	if bytes.Contains(file, []byte("synthetic")) {
		t.Fatal("plain text found in the sealed file")
	}
	dest := t.TempDir()
	m, err := Extract(bytes.NewReader(file), "correct horse battery", dest, folders)
	if err != nil || m.SchemaVersion != 97 {
		t.Fatalf("extract: %+v %v", m, err)
	}
	got, err := os.ReadFile(filepath.Join(dest, DumpName))
	if err != nil || !bytes.Equal(got, big) {
		t.Fatal("database dump changed", err)
	}
	if got, err = os.ReadFile(filepath.Join(dest, "uploads", "2026", "car.jpg")); err != nil || string(got) != "synthetic photo" {
		t.Fatal("photo changed", err)
	}
	if got, err = os.ReadFile(filepath.Join(dest, KeysName)); err != nil || string(got) != "DAPPER_MFA_KEY=synthetic\n" {
		t.Fatal("keys changed", err)
	}
}

func TestRefusesBadFiles(t *testing.T) {
	src, _ := sample(t)
	file := seal(t, src)
	flipped := bytes.Clone(file)
	flipped[len(file)/2] ^= 1
	for name, c := range map[string]struct {
		data []byte
		pass string
	}{
		"wrong passphrase": {file, "wrong horse battery"},
		"changed byte":     {flipped, "correct horse battery"},
		"cut short":        {file[:len(file)-2000], "correct horse battery"},
		"last chunk gone":  {file[:len(file)-(len(file)-len(magic)-saltSize)%(chunkSize+21)], "correct horse battery"},
		"not a backup":     {[]byte("just some other file that is long enough"), "correct horse battery"},
	} {
		if _, err := Extract(bytes.NewReader(c.data), c.pass, t.TempDir(), folders); !errors.Is(err, ErrUnreadable) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// A sealed archive with the right passphrase still cannot write outside its folder.
func TestRefusesPathEscape(t *testing.T) {
	for _, name := range []string{"../evil", "uploads/../../evil", "somewhere/else"} {
		var out bytes.Buffer
		salt := make([]byte, saltSize)
		gcm, err := newAEAD("pass", salt)
		if err != nil {
			t.Fatal(err)
		}
		out.WriteString(magic)
		out.Write(salt)
		s := &sealer{w: &out, gcm: gcm}
		tw := tar.NewWriter(s)
		if err = addBytes(tw, name, []byte("x")); err != nil {
			t.Fatal(err)
		}
		if err = tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err = s.flush(true); err != nil {
			t.Fatal(err)
		}
		dest := filepath.Join(t.TempDir(), "inner")
		if _, err = Extract(&out, "pass", dest, folders); !errors.Is(err, ErrUnreadable) {
			t.Fatalf("%s: %v", name, err)
		}
		if _, err = os.Stat(filepath.Join(filepath.Dir(dest), "evil")); err == nil {
			t.Fatalf("%s escaped", name)
		}
	}
}
