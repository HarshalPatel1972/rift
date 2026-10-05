package protocol

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"
)

func testKey() []byte {
	k := make([]byte, KeySize)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

// Vectors produced by the phone's vendored tweetnacl (web/phone/vendor), using
// the exact derivation and framing in web/phone/app.js. If these fail, the
// phone and the PC no longer speak the same protocol.
func TestInteropWithPhoneClient(t *testing.T) {
	keys := DeriveKeys(testKey())
	if got := hex.EncodeToString(keys.C2S[:]); got != "933995ab333b39f6b2a5a43be34db9bebaa13fed2670b9aefbe991dfdbe993af" {
		t.Fatalf("c2s key = %s", got)
	}
	if got := hex.EncodeToString(keys.S2C[:]); got != "9503700216a15a50caca43bf22ab0a1570d2b651e76573c8c6d82c8695b8e961" {
		t.Fatalf("s2c key = %s", got)
	}

	frame, _ := hex.DecodeString("6465666768696a6b6c6d6e6f707172737475767778797a7bb1ec6d5eaefeec18f2c74d520dc772c65d41a566b8f5eea0e539d8da101f")
	seq, op, payload, err := Open(&keys.C2S, frame)
	if err != nil {
		t.Fatal(err)
	}
	if seq != 1 || op != OpEdit || !bytes.Equal(payload, append([]byte{0, 0}, "hé😀"...)) {
		t.Fatalf("got seq=%d op=%#x payload=%q", seq, op, payload)
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	keys := DeriveKeys(testKey())
	w := Writer{Key: &keys.S2C}
	r := Reader{Key: &keys.S2C}
	for i := range 3 {
		op, p, err := r.Open(w.Seal(OpPing, []byte{byte(i)}))
		if err != nil || op != OpPing || p[0] != byte(i) {
			t.Fatalf("round %d: op=%#x p=%v err=%v", i, op, p, err)
		}
	}
}

func TestRejectsTamperingReplayAndReflection(t *testing.T) {
	keys := DeriveKeys(testKey())
	w := Writer{Key: &keys.C2S}
	f1 := w.Seal(OpKey, []byte{0x41, 0})
	f2 := w.Seal(OpKey, []byte{0x42, 0})

	r := Reader{Key: &keys.C2S}
	if _, _, err := r.Open(f2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Open(f1); !errors.Is(err, ErrReplay) {
		t.Fatalf("old frame accepted: %v", err)
	}
	if _, _, err := r.Open(f2); !errors.Is(err, ErrReplay) {
		t.Fatalf("duplicate frame accepted: %v", err)
	}

	bad := bytes.Clone(f1)
	bad[len(bad)-1] ^= 1
	if _, _, _, err := Open(&keys.C2S, bad); !errors.Is(err, ErrAuth) {
		t.Fatalf("tampered frame: %v", err)
	}

	// A client frame must not be accepted as a server frame.
	if _, _, _, err := Open(&keys.S2C, f1); !errors.Is(err, ErrAuth) {
		t.Fatalf("reflected frame: %v", err)
	}

	other := DeriveKeys(make([]byte, KeySize))
	if _, _, _, err := Open(&other.C2S, f1); !errors.Is(err, ErrAuth) {
		t.Fatalf("wrong key: %v", err)
	}

	if _, _, _, err := Open(&keys.C2S, f1[:10]); !errors.Is(err, ErrShort) {
		t.Fatalf("short frame: %v", err)
	}
}
