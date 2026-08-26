// Package tuya implements the Tuya LAN and Cloud protocol primitives.
package tuya

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/md5" //nolint:gosec // Tuya v3.4 protocol requires MD5
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
)

const (
	HeaderMagic = uint32(0x000055AA)
	FooterMagic = uint32(0x0000AA55)

	// Command codes.
	CmdSessKeyStart = uint32(3)  // v3.4 session key negotiation start
	CmdSessKeyResp  = uint32(4)  // v3.4 session key negotiation response
	CmdSessKeyFin   = uint32(5)  // v3.4 session key negotiation finish
	CmdControl      = uint32(7)  // SET DPS values
	CmdHeartbeat    = uint32(9)  // keepalive
	CmdDPQuery      = uint32(10) // GET status (0x0A)
	CmdDPQueryNew   = uint32(16) // alternate status query used by some devices
)

// Packet represents a decoded Tuya LAN packet.
type Packet struct {
	Seq     uint32
	Cmd     uint32
	RetCode uint32
	Payload []byte // decrypted JSON, may be empty for ACK packets
}

// ──────────────────────────────────────────────
// AES-128-ECB (no IV; Tuya's choice, not ours)
// ──────────────────────────────────────────────

// AESECBEncrypt encrypts data using AES-128-ECB with PKCS7 padding.
// key is truncated / zero-padded to 16 bytes.
func AESECBEncrypt(data, key []byte) ([]byte, error) {
	k := pad16(key)
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(data, aes.BlockSize)
	result := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(result[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}
	return result, nil
}

// AESECBDecrypt decrypts AES-128-ECB data and strips PKCS7 padding.
func AESECBDecrypt(data, key []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty ciphertext")
	}
	if len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a multiple of block size", len(data))
	}
	k := pad16(key)
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(result[i:i+aes.BlockSize], data[i:i+aes.BlockSize])
	}
	return pkcs7Unpad(result)
}

// ──────────────────────────────────────────────
// v3.4 session key derivation
// ──────────────────────────────────────────────

// DeriveSessionKey34 derives the v3.4 session key from local and remote nonces.
// remoteNonce is the plaintext 16-byte nonce received from the device.
// localNonce is the 16-byte nonce we sent.
//
// Mirrors tinytuya's _negotiate_session_key_generate_finalize: the session key
// is AES-128-ECB(encrypt) of (localNonce XOR remoteNonce) under the local key,
// with NO PKCS7 padding — a single 16-byte block.
func DeriveSessionKey34(localNonce, remoteNonce, localKey []byte) ([]byte, error) {
	if len(localNonce) != 16 || len(remoteNonce) != 16 {
		return nil, fmt.Errorf("nonces must be 16 bytes (local=%d remote=%d)", len(localNonce), len(remoteNonce))
	}

	// Session material is the byte-wise XOR of the two nonces.
	xored := make([]byte, 16)
	for i := 0; i < 16; i++ {
		xored[i] = localNonce[i] ^ remoteNonce[i]
	}

	// AES-128-ECB encrypt the 16-byte block under the local key, no padding.
	// AESECBEncrypt PKCS7-pads to 32 bytes; ECB blocks are independent, so the
	// first block is exactly the unpadded encryption of xored.
	encrypted, err := AESECBEncrypt(xored, localKey)
	if err != nil {
		return nil, err
	}
	return encrypted[:16], nil
}

// HMACKey34 returns the 16-byte MD5 of the local key used for pre-negotiation HMAC.
func HMACKey34(localKey []byte) []byte {
	//nolint:gosec // required by Tuya protocol v3.4
	h := md5.New()
	h.Write(localKey)
	return h.Sum(nil)
}

// HexKey returns the first 16 bytes of localKey as a hex string then converts back.
// Some devices use the hex-encoded key for their AES operations.
func HexKey(localKey []byte) []byte {
	hexStr := hex.EncodeToString(localKey[:min16(len(localKey))])
	return []byte(hexStr)[:16]
}

// ──────────────────────────────────────────────
// Packet building
// ──────────────────────────────────────────────

// BuildPacketV33 builds a complete v3.3 protocol packet.
// Payload is: "3.3" + 9×0x00 + AES-ECB-encrypt(jsonData, localKey).
func BuildPacketV33(seq, cmd uint32, jsonData, localKey []byte) ([]byte, error) {
	encrypted, err := AESECBEncrypt(jsonData, localKey)
	if err != nil {
		return nil, err
	}

	// Version prefix: "3.3" + 9 null bytes = 12 bytes.
	prefix := make([]byte, 12)
	copy(prefix, "3.3")
	payload := append(prefix, encrypted...)

	return wrapPacketCRC(seq, cmd, payload), nil
}

// v34HeaderlessCmds lists the commands that are NOT prefixed with the "3.4"
// protocol header before encryption (see tinytuya's NO_PROTOCOL_HEADER_CMDS).
// Everything else — notably CmdControl — gets the 15-byte header.
var v34HeaderlessCmds = map[uint32]bool{
	CmdSessKeyStart: true,
	CmdSessKeyResp:  true,
	CmdSessKeyFin:   true,
	CmdHeartbeat:    true,
	CmdDPQuery:      true,
	CmdDPQueryNew:   true,
}

// BuildPacketV34 builds a complete v3.4 protocol packet.
//
// Every v3.4 packet — including the SESS_KEY_NEG negotiation packets — is
// AES-128-ECB encrypted and HMAC-SHA256 framed under the same key: the local
// key during negotiation, the session key afterwards. Commands not in
// v34HeaderlessCmds (e.g. CmdControl) are prefixed with "3.4" + 12 null bytes
// before encryption.
func BuildPacketV34(seq, cmd uint32, jsonData, key []byte) ([]byte, error) {
	payload := jsonData
	if !v34HeaderlessCmds[cmd] {
		hdr := make([]byte, 15) // "3.4" + 12 × 0x00
		copy(hdr, "3.4")
		payload = append(hdr, payload...)
	}
	encrypted, err := AESECBEncrypt(payload, key)
	if err != nil {
		return nil, err
	}
	return wrapPacketHMAC(seq, cmd, encrypted, key), nil
}

// wrapPacketCRC wraps payload with the standard Tuya header/footer and CRC32.
func wrapPacketCRC(seq, cmd uint32, payload []byte) []byte {
	// len = len(payload) + 4 (CRC) + 4 (footer)
	length := uint32(len(payload) + 8)

	header := make([]byte, 16)
	binary.BigEndian.PutUint32(header[0:], HeaderMagic)
	binary.BigEndian.PutUint32(header[4:], seq)
	binary.BigEndian.PutUint32(header[8:], cmd)
	binary.BigEndian.PutUint32(header[12:], length)

	var buf bytes.Buffer
	buf.Write(header)
	buf.Write(payload)

	// CRC32 over everything so far.
	crc := crc32.ChecksumIEEE(buf.Bytes())
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc)
	buf.Write(crcBytes)

	footer := make([]byte, 4)
	binary.BigEndian.PutUint32(footer, FooterMagic)
	buf.Write(footer)

	return buf.Bytes()
}

// wrapPacketHMAC wraps payload with the standard header/footer and HMAC-SHA256.
func wrapPacketHMAC(seq, cmd uint32, payload, hmacKey []byte) []byte {
	// len = len(payload) + 32 (HMAC) + 4 (footer)
	length := uint32(len(payload) + 36)

	header := make([]byte, 16)
	binary.BigEndian.PutUint32(header[0:], HeaderMagic)
	binary.BigEndian.PutUint32(header[4:], seq)
	binary.BigEndian.PutUint32(header[8:], cmd)
	binary.BigEndian.PutUint32(header[12:], length)

	var buf bytes.Buffer
	buf.Write(header)
	buf.Write(payload)

	// HMAC-SHA256 over header+payload.
	h := hmac.New(sha256.New, hmacKey)
	h.Write(buf.Bytes())
	buf.Write(h.Sum(nil))

	footer := make([]byte, 4)
	binary.BigEndian.PutUint32(footer, FooterMagic)
	buf.Write(footer)

	return buf.Bytes()
}

// ──────────────────────────────────────────────
// Packet parsing
// ──────────────────────────────────────────────

// ParsePacketV33 decodes and decrypts a v3.3 response packet.
func ParsePacketV33(data, localKey []byte) (*Packet, error) {
	return parsePacket(data, localKey, false)
}

// ParsePacketV34 decodes and decrypts a v3.4 response packet using the session key.
func ParsePacketV34(data, sessionKey []byte) (*Packet, error) {
	return parsePacket(data, sessionKey, true)
}

func parsePacket(data, key []byte, isV34 bool) (*Packet, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(data))
	}

	// Validate magic bytes.
	hdr := binary.BigEndian.Uint32(data[0:4])
	if hdr != HeaderMagic {
		// Look for the magic in the buffer (device may send garbage first).
		idx := bytes.Index(data, []byte{0x00, 0x00, 0x55, 0xAA})
		if idx < 0 {
			return nil, fmt.Errorf("no valid packet header found")
		}
		data = data[idx:]
		if len(data) < 20 {
			return nil, fmt.Errorf("packet too short after sync: %d bytes", len(data))
		}
	}

	seq := binary.BigEndian.Uint32(data[4:8])
	cmd := binary.BigEndian.Uint32(data[8:12])
	length := binary.BigEndian.Uint32(data[12:16])

	var encrypted []byte
	var retCode uint32

	if isV34 {
		// v3.4: header(16) + encrypted_payload + hmac(32) + footer(4)
		// length = len(payload) + 32 + 4 = len(payload) + 36
		// TODO(security): verify HMAC-SHA256 over header+payload before trusting response.
		// The current implementation matches tinytuya's behavior (no response HMAC check),
		// but ideally we should reject packets with invalid HMACs to prevent spoofed LAN responses.
		if int(length) < 36 {
			return nil, fmt.Errorf("v3.4 packet length too small: %d", length)
		}
		payloadLen := int(length) - 36
		if 16+payloadLen > len(data) {
			return nil, fmt.Errorf("packet truncated: need %d, have %d", 16+payloadLen, len(data))
		}
		if payloadLen > 0 {
			encrypted = data[16 : 16+payloadLen]
		}
	} else {
		// v3.3: header(16) + payload + crc(4) + footer(4)
		// length = len(payload) + 8
		if int(length) < 8 {
			return nil, fmt.Errorf("v3.3 packet length too small: %d", length)
		}
		payloadLen := int(length) - 8
		if 16+payloadLen > len(data) {
			return nil, fmt.Errorf("packet truncated: need %d, have %d", 16+payloadLen, len(data))
		}
		if payloadLen > 0 {
			encrypted = data[16 : 16+payloadLen]
		}
	}

	var payload []byte
	if len(encrypted) > 0 {
		raw := encrypted

		if isV34 {
			// v3.4: may have 4-byte return code before encrypted data.
			if len(raw) > 4 {
				// Heuristic: if first 4 bytes look like a uint32 ≤ 255, treat as retcode.
				candidate := binary.BigEndian.Uint32(raw[:4])
				if candidate <= 0xFF {
					retCode = candidate
					raw = raw[4:]
				}
			}
		} else {
			// v3.3: strip version prefix if present.
			if len(raw) >= 12 && string(raw[:3]) == "3.3" {
				raw = raw[12:]
			} else if len(raw) >= 12 && string(raw[:3]) == "3.1" {
				raw = raw[12:]
			}
		}

		if len(raw) > 0 {
			var err error
			payload, err = AESECBDecrypt(raw, key)
			if err != nil {
				// Try without stripping prefix (some devices don't add it).
				payload, err = AESECBDecrypt(encrypted, key)
				if err != nil {
					return nil, fmt.Errorf("decrypt: %w", err)
				}
			}
		}
	}

	// v3.4 CONTROL responses may carry the "3.4" + 12-null protocol header ahead
	// of the plaintext; strip it so callers see the bare payload.
	if isV34 && len(payload) >= 15 && string(payload[:3]) == "3.4" {
		payload = payload[15:]
	}

	return &Packet{
		Seq:     seq,
		Cmd:     cmd,
		RetCode: retCode,
		Payload: payload,
	}, nil
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padByte := byte(padding)
	return append(data, bytes.Repeat([]byte{padByte}, padding)...)
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	padLen := int(data[len(data)-1])
	if padLen == 0 || padLen > aes.BlockSize {
		return nil, fmt.Errorf("invalid PKCS7 padding length: %d", padLen)
	}
	return data[:len(data)-padLen], nil
}

func pad16(key []byte) []byte {
	k := make([]byte, 16)
	copy(k, key)
	return k
}

func min16(n int) int {
	if n < 16 {
		return n
	}
	return 16
}
