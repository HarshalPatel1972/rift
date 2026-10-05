// Package protocol defines RIFT's encrypted binary wire format.
//
// Every WebSocket frame after the handshake challenge is:
//
//	nonce[24] || secretbox(plaintext)
//
// with plaintext:
//
//	seq u32 LE || op u8 || payload
//
// Each direction uses its own key derived from the 32-byte pairing key, so a
// frame can never be reflected back at its sender. Sequence numbers must be
// strictly increasing within a connection, which stops in-session replays;
// the handshake binds every connection to a fresh server challenge, which
// stops cross-session replays.
package protocol

import (
	"crypto/rand"
	"crypto/sha512"
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	Version       = 1
	KeySize       = 32
	NonceSize     = 24
	ChallengeSize = 16
	ClientNonceSz = 16
	headerSize    = 5 // seq u32 + op u8

	// MaxFrame bounds a single encrypted frame; clients chunk larger text.
	MaxFrame = 64 * 1024
)

// Magic prefixes the plaintext challenge frame: "RF" + version.
var Magic = [3]byte{'R', 'F', Version}

// Client -> server ops.
const (
	OpHello  byte = 0x01 // challenge[16] clientNonce[16] deviceName(utf8)
	OpEdit   byte = 0x02 // deleteCount u16 || insert(utf8)
	OpKey    byte = 0x03 // vk u8 || mods u8
	OpMove   byte = 0x04 // dx i16 || dy i16
	OpButton byte = 0x05 // button u8 || action u8
	OpScroll byte = 0x06 // dx i16 || dy i16 (wheel units, 120 = one notch)
	OpPong   byte = 0x07 // echo of the PING payload's timestamp (u64)
	OpAction byte = 0x08 // action u8 || arg u32 (see Action*)
	OpPeek   byte = 0x09 // mode u8 || maxWidth u16 — start/stop Screen Peek
	OpAck    byte = 0x0A // frameId u32 — phone is ready for the next frame
	OpPoint  byte = 0x0B // x u16 || y u16 || click u8 — absolute point on the last frame (0..65535)
)

// Server -> client ops.
const (
	OpWelcome byte = 0x81 // clientNonce[16] flags u8 hostName(utf8)
	OpPing    byte = 0x82 // ts u64 || rttMs u16 || flags u8 || timerSecs u32
	OpFrame   byte = 0x83 // frameId u32 || w u16 || h u16 || cursorX i16 || cursorY i16 || jpeg
)

// Flags carried in WELCOME / PING.
const (
	FlagPaused byte = 1 << 0
	FlagPeek   byte = 1 << 1 // host supports Screen Peek
	FlagPower  byte = 1 << 2 // host supports power actions
)

// Actions for OpAction.
const (
	ActionLock       byte = 1
	ActionDisplayOff byte = 2
	ActionSleep      byte = 3
	ActionSleepTimer byte = 4 // arg = minutes until sleep; 0 cancels
)

// Screen Peek modes for OpPeek.
const (
	PeekOff    byte = 0
	PeekScreen byte = 1 // the whole monitor the pointer is on
	PeekCursor byte = 2 // a crisp, unscaled window around the pointer
	PeekCaret  byte = 3 // around the text caret, falling back to the pointer
)

// MaxServerFrame bounds server->client frames (Screen Peek JPEGs).
const MaxServerFrame = 8 << 20

// Modifier bits for OpKey.
const (
	ModCtrl  byte = 1 << 0
	ModShift byte = 1 << 1
	ModAlt   byte = 1 << 2
	ModWin   byte = 1 << 3
)

// Mouse buttons and actions for OpButton.
const (
	ButtonLeft   byte = 0
	ButtonRight  byte = 1
	ButtonMiddle byte = 2

	ActionUp    byte = 0
	ActionDown  byte = 1
	ActionClick byte = 2
)

var (
	ErrShort    = errors.New("protocol: frame too short")
	ErrAuth     = errors.New("protocol: authentication failed")
	ErrReplay   = errors.New("protocol: sequence not increasing")
	ErrTooLarge = errors.New("protocol: frame too large")
)

// Keys holds the two directional subkeys derived from a pairing key.
type Keys struct {
	C2S [KeySize]byte
	S2C [KeySize]byte
}

// DeriveKeys splits a pairing key into per-direction keys:
// SHA-512(label || key)[:32]. The phone does the same with nacl.hash.
func DeriveKeys(pairing []byte) Keys {
	var k Keys
	c := sha512.Sum512(append([]byte("rift/c2s/v1"), pairing...))
	s := sha512.Sum512(append([]byte("rift/s2c/v1"), pairing...))
	copy(k.C2S[:], c[:KeySize])
	copy(k.S2C[:], s[:KeySize])
	return k
}

// Seal encrypts one message with a random nonce.
func Seal(key *[KeySize]byte, seq uint32, op byte, payload []byte) []byte {
	var nonce [NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	pt := make([]byte, headerSize+len(payload))
	binary.LittleEndian.PutUint32(pt, seq)
	pt[4] = op
	copy(pt[headerSize:], payload)
	return secretbox.Seal(nonce[:], pt, &nonce, key)
}

// Open authenticates and decrypts one frame.
func Open(key *[KeySize]byte, frame []byte) (seq uint32, op byte, payload []byte, err error) {
	// Inbound client frames are capped at MaxFrame by the socket read limit.
	if len(frame) > MaxServerFrame {
		return 0, 0, nil, ErrTooLarge
	}
	if len(frame) < NonceSize+secretbox.Overhead+headerSize {
		return 0, 0, nil, ErrShort
	}
	var nonce [NonceSize]byte
	copy(nonce[:], frame[:NonceSize])
	pt, ok := secretbox.Open(nil, frame[NonceSize:], &nonce, key)
	if !ok {
		return 0, 0, nil, ErrAuth
	}
	return binary.LittleEndian.Uint32(pt), pt[4], pt[headerSize:], nil
}

// Reader enforces strictly increasing sequence numbers for one direction.
type Reader struct {
	Key  *[KeySize]byte
	last uint32
}

func (r *Reader) Open(frame []byte) (op byte, payload []byte, err error) {
	seq, op, payload, err := Open(r.Key, frame)
	if err != nil {
		return 0, nil, err
	}
	if seq <= r.last {
		return 0, nil, ErrReplay
	}
	r.last = seq
	return op, payload, nil
}

// Writer stamps outgoing frames with increasing sequence numbers.
type Writer struct {
	Key *[KeySize]byte
	seq uint32
}

func (w *Writer) Seal(op byte, payload []byte) []byte {
	w.seq++
	return Seal(w.Key, w.seq, op, payload)
}

// I16 reads a little-endian int16 at offset off.
func I16(b []byte, off int) int {
	return int(int16(binary.LittleEndian.Uint16(b[off:])))
}
