// Package backup writes and reads the encrypted backup file: a tar archive sealed with a passphrase that is never stored.
package backup

import (
	"archive/tar"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	magic     = "DAPPERBK1"
	chunkSize = 1 << 20
	saltSize  = 16
)

// ErrUnreadable means the passphrase is wrong, or the file is damaged or not one of ours. The encryption cannot tell these apart.
var ErrUnreadable = errors.New("wrong passphrase or damaged backup file")

const (
	manifestName = "manifest.json"
	DumpName     = "database.dump"
	KeysName     = "keys.env"
)

// Manifest says what wrote the file. A file from a different schema must not be restored.
type Manifest struct {
	SchemaVersion int       `json:"schemaVersion"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Source is what goes into a backup. Folders maps an archive folder name to where it lives on disk.
type Source struct {
	DumpPath string
	Keys     []byte
	Folders  map[string]string
}

func newAEAD(passphrase string, salt []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 4, 32))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// The key is unique to each file (random salt), so a chunk counter is a safe nonce. The last byte marks the final chunk, so cutting a file short is detected.
func chunkNonce(gcm cipher.AEAD, n uint64, final bool) []byte {
	nonce := make([]byte, gcm.NonceSize())
	binary.BigEndian.PutUint64(nonce[3:11], n)
	if final {
		nonce[11] = 1
	}
	return nonce
}

type sealer struct {
	w   io.Writer
	gcm cipher.AEAD
	buf []byte
	n   uint64
}

func (s *sealer) flush(final bool) error {
	sealed := s.gcm.Seal(nil, chunkNonce(s.gcm, s.n, final), s.buf, nil)
	header := make([]byte, 5)
	if final {
		header[0] = 1
	}
	binary.BigEndian.PutUint32(header[1:], uint32(len(sealed)))
	if _, err := s.w.Write(header); err != nil {
		return err
	}
	if _, err := s.w.Write(sealed); err != nil {
		return err
	}
	s.n++
	s.buf = s.buf[:0]
	return nil
}

func (s *sealer) Write(p []byte) (int, error) {
	total := len(p)
	for len(p) > 0 {
		take := min(chunkSize-len(s.buf), len(p))
		s.buf = append(s.buf, p[:take]...)
		p = p[take:]
		if len(s.buf) == chunkSize {
			if err := s.flush(false); err != nil {
				return 0, err
			}
		}
	}
	return total, nil
}

type opener struct {
	r    io.Reader
	gcm  cipher.AEAD
	buf  []byte
	n    uint64
	done bool
}

func (o *opener) Read(p []byte) (int, error) {
	for len(o.buf) == 0 {
		if o.done {
			return 0, io.EOF
		}
		header := make([]byte, 5)
		if _, err := io.ReadFull(o.r, header); err != nil {
			return 0, ErrUnreadable
		}
		size := binary.BigEndian.Uint32(header[1:])
		if size > chunkSize+uint32(o.gcm.Overhead()) {
			return 0, ErrUnreadable
		}
		sealed := make([]byte, size)
		if _, err := io.ReadFull(o.r, sealed); err != nil {
			return 0, ErrUnreadable
		}
		final := header[0] == 1
		plain, err := o.gcm.Open(nil, chunkNonce(o.gcm, o.n, final), sealed, nil)
		if err != nil {
			return 0, ErrUnreadable
		}
		o.n++
		o.buf, o.done = plain, final
	}
	n := copy(p, o.buf)
	o.buf = o.buf[n:]
	return n, nil
}

func addBytes(tw *tar.Writer, name string, data []byte) error {
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func addFile(tw *tar.Writer, name, source string) error {
	f, err := os.Open(source)
	if errors.Is(err, fs.ErrNotExist) { // removed since the folder was listed, such as an expired key-tag photo
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: info.Size(), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

// Write seals a complete backup into w.
func Write(w io.Writer, passphrase string, m Manifest, src Source) error {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	gcm, err := newAEAD(passphrase, salt)
	if err != nil {
		return err
	}
	if _, err = w.Write(append([]byte(magic), salt...)); err != nil {
		return err
	}
	sealed := &sealer{w: w, gcm: gcm, buf: make([]byte, 0, chunkSize)}
	tw := tar.NewWriter(sealed)
	manifest, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err = addBytes(tw, manifestName, manifest); err != nil {
		return err
	}
	if err = addFile(tw, DumpName, src.DumpPath); err != nil {
		return err
	}
	if err = addBytes(tw, KeysName, src.Keys); err != nil {
		return err
	}
	for folder, dir := range src.Folders {
		err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if errors.Is(walkErr, fs.ErrNotExist) {
					return nil
				}
				return walkErr
			}
			if !d.Type().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			return addFile(tw, path.Join(folder, filepath.ToSlash(rel)), p)
		})
		if err != nil {
			return err
		}
	}
	if err = tw.Close(); err != nil {
		return err
	}
	return sealed.flush(true)
}

// Extract opens a backup into dest and returns its manifest. A nil error means every chunk passed its check to the end of the file; until then
// nothing outside dest has been touched.
func Extract(r io.Reader, passphrase, dest string, folders []string) (Manifest, error) {
	head := make([]byte, len(magic)+saltSize)
	if _, err := io.ReadFull(r, head); err != nil || !bytes.Equal(head[:len(magic)], []byte(magic)) {
		return Manifest{}, ErrUnreadable
	}
	gcm, err := newAEAD(passphrase, head[len(magic):])
	if err != nil {
		return Manifest{}, err
	}
	opened := &opener{r: r, gcm: gcm}
	tr := tar.NewReader(opened)
	var manifest *Manifest
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.Is(err, ErrUnreadable) {
				return Manifest{}, ErrUnreadable
			}
			return Manifest{}, ErrUnreadable
		}
		if h.Typeflag != tar.TypeReg || !allowedEntry(h.Name, folders) {
			return Manifest{}, ErrUnreadable
		}
		if h.Name == manifestName {
			var m Manifest
			if err = json.NewDecoder(tr).Decode(&m); err != nil {
				return Manifest{}, ErrUnreadable
			}
			manifest = &m
			continue
		}
		target := filepath.Join(dest, filepath.FromSlash(h.Name))
		if err = os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return Manifest{}, err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return Manifest{}, err
		}
		_, err = io.Copy(f, tr)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			if errors.Is(err, ErrUnreadable) {
				return Manifest{}, ErrUnreadable
			}
			return Manifest{}, err
		}
	}
	// tar stops at its own end mark; the final chunk still has to be read and checked.
	if _, err = io.Copy(io.Discard, opened); err != nil || manifest == nil || !opened.done {
		return Manifest{}, ErrUnreadable
	}
	return *manifest, nil
}

// allowedEntry keeps a crafted archive from writing anywhere but the known names.
func allowedEntry(name string, folders []string) bool {
	if !filepath.IsLocal(filepath.FromSlash(name)) {
		return false
	}
	if name == manifestName || name == DumpName || name == KeysName {
		return true
	}
	for _, folder := range folders {
		if strings.HasPrefix(name, folder+"/") {
			return true
		}
	}
	return false
}
