package tuya_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/yashiels/orro-cli/internal/tuya"
)

func TestAESECBRoundtrip(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	plaintext := []byte(`{"devId":"test","t":"1234567890"}`)

	encrypted, err := tuya.AESECBEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("AESECBEncrypt error: %v", err)
	}
	if len(encrypted)%16 != 0 {
		t.Errorf("encrypted length %d is not a multiple of 16", len(encrypted))
	}

	decrypted, err := tuya.AESECBDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("AESECBDecrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestAESECBShortKey(t *testing.T) {
	key := []byte("shortkey") // 8 bytes, should be padded to 16
	plaintext := []byte("hello world!!!!!")

	encrypted, err := tuya.AESECBEncrypt(plaintext, key)
	if err != nil {
		t.Fatalf("AESECBEncrypt error: %v", err)
	}
	decrypted, err := tuya.AESECBDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("AESECBDecrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip with short key failed")
	}
}

func TestBuildPacketV33Structure(t *testing.T) {
	key := []byte("0123456789abcdef")
	payload := []byte(`{"gwId":"abc","devId":"abc","uid":"abc","t":"1000"}`)
	seq := uint32(1)

	pkt, err := tuya.BuildPacketV33(seq, tuya.CmdDPQuery, payload, key)
	if err != nil {
		t.Fatalf("BuildPacketV33 error: %v", err)
	}

	// Verify header magic.
	header := binary.BigEndian.Uint32(pkt[0:4])
	if header != tuya.HeaderMagic {
		t.Errorf("header = 0x%08X, want 0x%08X", header, tuya.HeaderMagic)
	}

	// Verify footer magic.
	footer := binary.BigEndian.Uint32(pkt[len(pkt)-4:])
	if footer != tuya.FooterMagic {
		t.Errorf("footer = 0x%08X, want 0x%08X", footer, tuya.FooterMagic)
	}

	// Verify sequence number.
	seqGot := binary.BigEndian.Uint32(pkt[4:8])
	if seqGot != seq {
		t.Errorf("seq = %d, want %d", seqGot, seq)
	}

	// Verify command.
	cmdGot := binary.BigEndian.Uint32(pkt[8:12])
	if cmdGot != tuya.CmdDPQuery {
		t.Errorf("cmd = %d, want %d", cmdGot, tuya.CmdDPQuery)
	}

	// Minimum sensible length.
	if len(pkt) < 24 {
		t.Errorf("packet too short: %d bytes", len(pkt))
	}
}

func TestParsePacketV33Roundtrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	originalPayload := []byte(`{"devId":"abc","dps":{"150":true},"t":"1000","uid":"abc"}`)
	seq := uint32(42)

	pkt, err := tuya.BuildPacketV33(seq, tuya.CmdControl, originalPayload, key)
	if err != nil {
		t.Fatalf("BuildPacketV33 error: %v", err)
	}

	// Simulate a response by adjusting the packet (normally device would encrypt differently,
	// but we can test our parser by feeding back our own packet with the version prefix stripped).
	parsed, err := tuya.ParsePacketV33(pkt, key)
	if err != nil {
		t.Fatalf("ParsePacketV33 error: %v", err)
	}

	if parsed.Seq != seq {
		t.Errorf("parsed seq = %d, want %d", parsed.Seq, seq)
	}
	if parsed.Cmd != tuya.CmdControl {
		t.Errorf("parsed cmd = %d, want %d", parsed.Cmd, tuya.CmdControl)
	}
	if !bytes.Equal(parsed.Payload, originalPayload) {
		t.Errorf("parsed payload = %q, want %q", parsed.Payload, originalPayload)
	}
}

func TestDeriveSessionKey34(t *testing.T) {
	localKey := []byte("0123456789abcdef")
	localNonce := bytes.Repeat([]byte{0x01}, 16)
	remoteNonce := bytes.Repeat([]byte{0x02}, 16)

	sessionKey, err := tuya.DeriveSessionKey34(localNonce, remoteNonce, localKey)
	if err != nil {
		t.Fatalf("DeriveSessionKey34 error: %v", err)
	}
	if len(sessionKey) != 16 {
		t.Errorf("session key length = %d, want 16", len(sessionKey))
	}

	// Deterministic — same inputs should produce same key.
	sessionKey2, err := tuya.DeriveSessionKey34(localNonce, remoteNonce, localKey)
	if err != nil {
		t.Fatalf("DeriveSessionKey34 error: %v", err)
	}
	if !bytes.Equal(sessionKey, sessionKey2) {
		t.Error("DeriveSessionKey34 is not deterministic")
	}
}

func TestBuildPacketV34Structure(t *testing.T) {
	sessionKey := []byte("sessionkey123456") // 16 bytes
	payload := []byte(`{"devId":"abc","dps":{"150":true},"t":"1000","uid":"abc"}`)
	seq := uint32(3)

	pkt, err := tuya.BuildPacketV34(seq, tuya.CmdControl, payload, sessionKey)
	if err != nil {
		t.Fatalf("BuildPacketV34 error: %v", err)
	}

	// Header magic.
	if binary.BigEndian.Uint32(pkt[0:4]) != tuya.HeaderMagic {
		t.Error("v3.4 packet has wrong header magic")
	}
	// Footer magic.
	if binary.BigEndian.Uint32(pkt[len(pkt)-4:]) != tuya.FooterMagic {
		t.Error("v3.4 packet has wrong footer magic")
	}
	// v3.4 has 32-byte HMAC before footer.
	// Total = 16 (header) + encrypted_payload + 32 (HMAC) + 4 (footer).
	// Must be at least 52 bytes.
	if len(pkt) < 52 {
		t.Errorf("v3.4 packet too short: %d bytes", len(pkt))
	}
}

func TestHMACKey34(t *testing.T) {
	key := []byte("testkey")
	hmacKey := tuya.HMACKey34(key)
	if len(hmacKey) != 16 {
		t.Errorf("HMAC key length = %d, want 16 (MD5)", len(hmacKey))
	}
}
