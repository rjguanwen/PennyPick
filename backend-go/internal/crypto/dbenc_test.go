package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	plain := []byte("SQLite format 3\x00hello pennypick data")
	enc, err := EncryptBytes(plain, "secret-password")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if !bytes.HasPrefix(enc, []byte(Magic)) {
		t.Fatal("missing magic prefix")
	}
	if len(enc) != HeaderLen+len(plain)+16 {
		t.Fatalf("unexpected ciphertext length: %d", len(enc))
	}
	got, err := DecryptBytes(enc, "secret-password")
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("round trip mismatch")
	}
}

func TestWrongPassword(t *testing.T) {
	enc, err := EncryptBytes([]byte("data"), "right")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := DecryptBytes(enc, "wrong"); err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestBadMagic(t *testing.T) {
	if _, err := DecryptBytes([]byte("SQLite format 3\x00..."), "x"); err == nil {
		t.Fatal("expected bad magic error")
	}
	if _, err := DecryptBytes(nil, "x"); err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestEncryptFileAndDetect(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.db")
	dst := filepath.Join(dir, "a.db.enc")
	if err := os.WriteFile(src, []byte("SQLite format 3\x00data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EncryptFile(src, dst, "pass"); err != nil {
		t.Fatal(err)
	}
	ok, err := IsEncryptedFile(dst)
	if err != nil || !ok {
		t.Fatalf("IsEncryptedFile(dst): ok=%v err=%v", ok, err)
	}
	ok, err = IsEncryptedFile(src)
	if err != nil || ok {
		t.Fatalf("IsEncryptedFile(src): ok=%v err=%v", ok, err)
	}
	ok, err = IsEncryptedFile(filepath.Join(dir, "nope.db"))
	if err != nil || ok {
		t.Fatalf("IsEncryptedFile(missing): ok=%v err=%v", ok, err)
	}
	plain, err := DecryptFile(dst, "pass")
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "SQLite format 3\x00data" {
		t.Fatalf("mismatch: %q", plain)
	}
}

func TestSamePasswordDifferentCiphertext(t *testing.T) {
	a, _ := EncryptBytes([]byte("payload"), "same")
	b, _ := EncryptBytes([]byte("payload"), "same")
	if bytes.Equal(a, b) {
		t.Fatal("encryptions should be randomized (different salt/nonce)")
	}
	pa, err := DecryptBytes(a, "same")
	if err != nil {
		t.Fatal(err)
	}
	pb, err := DecryptBytes(b, "same")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pa, pb) {
		t.Fatal("decrypted payloads should match")
	}
}
