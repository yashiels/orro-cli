package tuya

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yashiels/orro-cli/internal/config"
)

const (
	lanPort       = 6668
	discoverPort  = 6666 // UDP discovery (v3.3)
	discoverPort2 = 6667 // UDP discovery (v3.4+)
	tcpTimeout    = 5 * time.Second
	readTimeout   = 5 * time.Second
	readBufSize   = 4096
)

// LAN controls a Tuya device directly over the local network.
type LAN struct {
	cfg        *config.Config
	ip         string
	version    string
	localKey   []byte
	sessionKey []byte // non-nil after v3.4 negotiation
	conn       net.Conn
	seq        uint32
}

// NewLAN creates a LAN client, discovering the device IP if not configured.
func NewLAN(cfg *config.Config) (*LAN, error) {
	if cfg.LocalKey == "" {
		return nil, fmt.Errorf(
			"no local key configured — set ORRO_LOCAL_KEY, add 'local_key' to config.toml, " +
				"or add 'Local Key' to the 1Password item",
		)
	}

	l := &LAN{
		cfg:      cfg,
		ip:       cfg.LANIP,
		version:  cfg.LANVersion,
		localKey: []byte(cfg.LocalKey),
	}

	if l.ip == "" {
		cfg.Debug("lan: no IP configured, attempting discovery")
		ip, ver, err := discover(cfg.DeviceID, 3*time.Second)
		if err != nil || ip == "" {
			return nil, fmt.Errorf("could not discover LAN IP for device: %v", err)
		}
		l.ip = ip
		if ver != "" && l.version == "" {
			l.version = ver
		}
		cfg.Debug(fmt.Sprintf("lan: discovered %s at %s", cfg.DeviceID, ip))
	}

	if err := l.connect(); err != nil {
		return nil, err
	}
	return l, nil
}

// Status returns the current device status DPs.
func (l *LAN) Status() (map[string]any, error) {
	payload := l.statusPayload()
	pkt, err := l.sendRecv(CmdDPQuery, payload)
	if err != nil {
		return nil, fmt.Errorf("lan status: %w", err)
	}
	var result map[string]any
	if err := json.Unmarshal(pkt.Payload, &result); err != nil {
		return nil, fmt.Errorf("lan status decode: %w (raw: %s)", err, pkt.Payload)
	}
	return result, nil
}

// SendCodes sends a list of {code, value} commands using the configured DP map.
func (l *LAN) SendCodes(commands []Command) error {
	dps := make(map[string]any, len(commands))
	for _, cmd := range commands {
		dp, ok := l.cfg.LANDPs[cmd.Code]
		if !ok {
			return fmt.Errorf("no LAN DP mapping for code %q", cmd.Code)
		}
		dps[strconv.Itoa(dp)] = cmd.Value
	}

	jsonPayload := map[string]any{
		"devId": l.cfg.DeviceID,
		"uid":   l.cfg.DeviceID,
		"t":     strconv.FormatInt(time.Now().Unix(), 10),
		"dps":   dps,
	}
	payload, err := json.Marshal(jsonPayload)
	if err != nil {
		return err
	}

	l.cfg.Debug(fmt.Sprintf("lan: sending %s", payload))
	_, err = l.sendRecv(CmdControl, payload)
	return err
}

// Close shuts down the TCP connection.
func (l *LAN) Close() {
	if l.conn != nil {
		_ = l.conn.Close()
	}
}

// IP returns the device IP address.
func (l *LAN) IP() string { return l.ip }

// Version returns the negotiated protocol version.
func (l *LAN) Version() string { return l.version }

// ──────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────

func (l *LAN) connect() error {
	addr := net.JoinHostPort(l.ip, fmt.Sprintf("%d", lanPort))
	l.cfg.Debug(fmt.Sprintf("lan: connecting to %s", addr))

	versions := l.candidateVersions()
	var lastErr error

	for _, ver := range versions {
		l.cfg.Debug(fmt.Sprintf("lan: trying protocol v%s", ver))
		conn, err := net.DialTimeout("tcp", addr, tcpTimeout)
		if err != nil {
			lastErr = fmt.Errorf("connect %s: %w", addr, err)
			continue
		}
		l.conn = conn

		isV34 := strings.HasPrefix(ver, "3.4") || strings.HasPrefix(ver, "3.5")
		if isV34 {
			if err := l.negotiateSession(); err != nil {
				_ = conn.Close()
				l.conn = nil
				lastErr = fmt.Errorf("v3.4 session negotiation: %w", err)
				continue
			}
		}

		// Probe with a status query.
		if _, err := l.Status(); err != nil {
			_ = conn.Close()
			l.conn = nil
			l.sessionKey = nil
			lastErr = fmt.Errorf("v%s probe failed: %w", ver, err)
			continue
		}

		l.version = ver
		l.cfg.LANVersion = ver
		l.cfg.Debug(fmt.Sprintf("lan: connected with v%s", ver))
		return nil
	}

	return fmt.Errorf("all protocol versions failed for %s: %v", l.ip, lastErr)
}

func (l *LAN) candidateVersions() []string {
	var versions []string
	if l.version != "" {
		versions = append(versions, l.version)
	}
	for _, v := range []string{"3.4", "3.3", "3.5", "3.1"} {
		if v != l.version {
			versions = append(versions, v)
		}
	}
	return versions
}

// negotiateSession performs the v3.4 session key exchange.
func (l *LAN) negotiateSession() error {
	// Step 1: generate a random 16-byte local nonce.
	localNonce := make([]byte, 16)
	if _, err := rand.Read(localNonce); err != nil {
		return err
	}

	// Step 2: encrypt local nonce with local key.
	encNonce, err := AESECBEncrypt(localNonce, l.localKey)
	if err != nil {
		return fmt.Errorf("encrypt nonce: %w", err)
	}

	// Step 3: send SESS_KEY_NEG_START.
	seq := atomic.AddUint32(&l.seq, 1)
	pkt := BuildPreNegPacketV34(seq, CmdSessKeyStart, encNonce)
	l.cfg.Debug("lan: sending SESS_KEY_NEG_START")
	if _, err := l.conn.Write(pkt); err != nil {
		return fmt.Errorf("write SESS_KEY_NEG_START: %w", err)
	}

	// Step 4: read SESS_KEY_NEG_RESP.
	raw, err := l.readRaw()
	if err != nil {
		return fmt.Errorf("read SESS_KEY_NEG_RESP: %w", err)
	}
	resp, err := ParsePreNegPacket(raw)
	if err != nil {
		return fmt.Errorf("parse SESS_KEY_NEG_RESP: %w", err)
	}
	if resp.Cmd != CmdSessKeyResp {
		return fmt.Errorf("expected SESS_KEY_NEG_RESP (cmd=%d), got cmd=%d", CmdSessKeyResp, resp.Cmd)
	}

	// Step 5: decrypt device's remote nonce.
	remoteNonce, err := AESECBDecrypt(resp.Payload, l.localKey)
	if err != nil {
		return fmt.Errorf("decrypt remote nonce: %w", err)
	}

	// Step 6: derive session key.
	sessionKey, err := DeriveSessionKey34(localNonce, remoteNonce, l.localKey)
	if err != nil {
		return fmt.Errorf("derive session key: %w", err)
	}
	l.sessionKey = sessionKey
	l.cfg.Debug("lan: session key derived")

	// Step 7: send SESS_KEY_NEG_FINISH — HMAC-SHA256(remoteNonce, key=sessionKey).
	h := hmac.New(sha256.New, sessionKey)
	h.Write(remoteNonce)
	finPayload := h.Sum(nil)

	seq2 := atomic.AddUint32(&l.seq, 1)
	finPkt := wrapPacketHMAC(seq2, CmdSessKeyFin, finPayload, sessionKey)
	l.cfg.Debug("lan: sending SESS_KEY_NEG_FINISH")
	if _, err := l.conn.Write(finPkt); err != nil {
		return fmt.Errorf("write SESS_KEY_NEG_FINISH: %w", err)
	}

	// Read acknowledgement (optional; some devices don't send one).
	_ = l.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, readBufSize)
	n, _ := l.conn.Read(buf)
	_ = l.conn.SetReadDeadline(time.Time{})
	l.cfg.Debug(fmt.Sprintf("lan: SESS_KEY_NEG_FINISH ack: %d bytes", n))

	return nil
}

func (l *LAN) sendRecv(cmd uint32, payload []byte) (*Packet, error) {
	seq := atomic.AddUint32(&l.seq, 1)

	var wire []byte
	var err error

	isV34 := l.sessionKey != nil
	if isV34 {
		wire, err = BuildPacketV34(seq, cmd, payload, l.sessionKey)
	} else {
		wire, err = BuildPacketV33(seq, cmd, payload, l.localKey)
	}
	if err != nil {
		return nil, fmt.Errorf("build packet: %w", err)
	}

	l.cfg.Debug(fmt.Sprintf("lan: send cmd=%d seq=%d", cmd, seq))
	if _, err := l.conn.Write(wire); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	raw, err := l.readRaw()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if isV34 {
		return ParsePacketV34(raw, l.sessionKey)
	}
	return ParsePacketV33(raw, l.localKey)
}

func (l *LAN) readRaw() ([]byte, error) {
	_ = l.conn.SetReadDeadline(time.Now().Add(readTimeout))
	defer func() { _ = l.conn.SetReadDeadline(time.Time{}) }()

	var buf bytes.Buffer
	tmp := make([]byte, readBufSize)

	for {
		n, err := l.conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			// Timeout means no more data.
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			if buf.Len() == 0 {
				return nil, err
			}
			break
		}
		// Stop after we have a complete packet (header magic + footer magic present).
		data := buf.Bytes()
		if bytes.Contains(data, []byte{0x00, 0x00, 0xAA, 0x55}) {
			break
		}
	}
	return buf.Bytes(), nil
}

func (l *LAN) statusPayload() []byte {
	p := map[string]string{
		"gwId":  l.cfg.DeviceID,
		"devId": l.cfg.DeviceID,
		"uid":   l.cfg.DeviceID,
		"t":     strconv.FormatInt(time.Now().Unix(), 10),
	}
	b, _ := json.Marshal(p)
	return b
}

// ──────────────────────────────────────────────
// UDP device discovery
// ──────────────────────────────────────────────

// DiscoveryResult is returned by Discover.
type DiscoveryResult struct {
	IP      string
	DevID   string
	Version string
}

// discover listens for Tuya UDP broadcasts and returns the IP for the given device ID.
func discover(devID string, timeout time.Duration) (ip, version string, err error) {
	results := make(chan DiscoveryResult, 4)
	done := make(chan struct{})

	// Listen on both ports in parallel.
	for _, port := range []int{discoverPort, discoverPort2} {
		go listenUDP(port, devID, results, done)
	}

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	defer close(done)

	select {
	case r := <-results:
		return r.IP, r.Version, nil
	case <-deadline.C:
		return "", "", fmt.Errorf("device %s not found within %s", devID, timeout)
	}
}

func listenUDP(port int, devID string, results chan<- DiscoveryResult, done <-chan struct{}) {
	addr := &net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	for {
		select {
		case <-done:
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}

		r, ok := parseDiscoveryBroadcast(buf[:n], devID)
		if ok {
			select {
			case results <- r:
			default:
			}
		}
	}
}

// parseDiscoveryBroadcast attempts to extract device info from a raw UDP broadcast.
// Tuya broadcasts are Tuya-encrypted JSON or plain JSON.
func parseDiscoveryBroadcast(data []byte, targetDevID string) (DiscoveryResult, bool) {
	// Try plain JSON first.
	var msg map[string]any
	if err := json.Unmarshal(data, &msg); err == nil {
		return extractFromJSON(msg, targetDevID)
	}

	// Try to find JSON within the broadcast (skip binary header).
	start := bytes.IndexByte(data, '{')
	if start >= 0 {
		if err := json.Unmarshal(data[start:], &msg); err == nil {
			return extractFromJSON(msg, targetDevID)
		}
	}

	return DiscoveryResult{}, false
}

func extractFromJSON(msg map[string]any, targetDevID string) (DiscoveryResult, bool) {
	gwID, _ := msg["gwId"].(string)
	devID, _ := msg["devId"].(string)
	ip, _ := msg["ip"].(string)
	version, _ := msg["version"].(string)

	id := gwID
	if id == "" {
		id = devID
	}

	if targetDevID != "" && id != targetDevID {
		return DiscoveryResult{}, false
	}
	if ip == "" {
		return DiscoveryResult{}, false
	}
	return DiscoveryResult{IP: ip, DevID: id, Version: version}, true
}
