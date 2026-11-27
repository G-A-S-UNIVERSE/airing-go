package zte

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/G-A-S-UNIVERSE/airing-go/internal/olt"
)

func init() {
	olt.Register("zte_c320", func() olt.BaseOLTAdapter {
		return &ZteC320Adapter{port: 23, timeout: 15 * time.Second}
	})
}

// ErrReadOnlyMode indicates write/reboot operations are permanently blocked by strict read-only policy.
var ErrReadOnlyMode = errors.New("zte_c320: write/reboot disabled (strict read-only mode)")

var (
	rePromptLine    = regexp.MustCompile(`(?m)^[A-Za-z][A-Za-z0-9_.\-]*[#>\]]\s*$`)
	reONUID         = regexp.MustCompile(`^\d+/\d+/\d+:\d+$`)
	reONUStateLine  = regexp.MustCompile(`^(\d+/\d+/\d+):(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)`)
	reCardLine      = regexp.MustCompile(`^\d+\s+\d+\s+\d+\s+\S+.*?\s+(INSERVICE|OFFLINE|STANDBY|FAIL|HWONLINE)\s*$`)
	reOpticalValue  = regexp.MustCompile(`(-?\d+(?:\.\d+)?)\s*\(dbm\)`)
	reTelnetPrompts = struct {
		user *regexp.Regexp
		pass *regexp.Regexp
		err  *regexp.Regexp
	}{
		user: regexp.MustCompile(`(?i)(username|user name|login)\s*:\s*$`),
		pass: regexp.MustCompile(`(?i)password\s*:\s*$`),
		err:  regexp.MustCompile(`(?i)(authentication failed|invalid username|login failed)`),
	}
)

// ValidateReadOnlyCommand enforces an absolute CLI allowlist so write/config commands can never be sent.
func ValidateReadOnlyCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return errors.New("zte_c320: empty command rejected")
	}
	if strings.ContainsAny(trimmed, "\r\n;|") {
		return errors.New("zte_c320: multi-command characters rejected")
	}
	lower := strings.ToLower(trimmed)
	if lower == "terminal length 0" || lower == "exit" {
		return nil
	}
	if strings.HasPrefix(lower, "show ") {
		return nil
	}
	return fmt.Errorf("zte_c320: command %q blocked by strict read-only allowlist", trimmed)
}

func validateONUID(onuID string) error {
	clean := strings.TrimSpace(onuID)
	if !reONUID.MatchString(clean) {
		return fmt.Errorf("zte_c320: invalid onu_id %q (expected rack/slot/port:index)", onuID)
	}
	return nil
}

// Config holds ZTE C320 Telnet CLI connection parameters.
type Config struct {
	Host     string
	Port     int // default 23
	Username string
	Password string
	Timeout  time.Duration // default 15s
}

// ZteC320Adapter reads ZTE C320 OLT telemetry over Telnet CLI in strict read-only mode.
type ZteC320Adapter struct {
	host    string
	port    int
	user    string
	pass    string
	timeout time.Duration
}

// New creates a ZteC320Adapter from Config.
func New(cfg Config) *ZteC320Adapter {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	port := cfg.Port
	if port <= 0 {
		port = 23
	}
	return &ZteC320Adapter{
		host:    strings.TrimSpace(cfg.Host),
		port:    port,
		user:    cfg.Username,
		pass:    cfg.Password,
		timeout: timeout,
	}
}

func (a *ZteC320Adapter) VendorKey() string { return "zte_c320" }

func (a *ZteC320Adapter) Connect(ctx context.Context) error {
	if a.host == "" {
		return errors.New("zte_c320: host not configured")
	}
	addr := net.JoinHostPort(a.host, strconv.Itoa(a.port))
	d := net.Dialer{Timeout: a.timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("zte_c320: connect to %s: %w", addr, err)
	}
	_ = conn.Close()
	return nil
}

func (a *ZteC320Adapter) Disconnect(_ context.Context) error { return nil }

// RebootONU is permanently disabled under strict read-only policy.
func (a *ZteC320Adapter) RebootONU(_ context.Context, _ string) error {
	return ErrReadOnlyMode
}

func (a *ZteC320Adapter) ListONUs(ctx context.Context) ([]olt.ONUStatus, error) {
	outputs, err := a.execReadOnly(ctx, []string{"show gpon onu state"})
	if err != nil {
		return nil, err
	}
	return ParseONUStateOutput(outputs["show gpon onu state"]), nil
}

func (a *ZteC320Adapter) GetONU(ctx context.Context, onuID string) (*olt.ONUStatus, error) {
	if err := validateONUID(onuID); err != nil {
		return nil, err
	}
	cleanID := strings.TrimSpace(onuID)
	cmdState := "show gpon onu state gpon-onu_" + cleanID
	cmdPower := "show pon power onu-rx gpon-onu_" + cleanID
	outputs, err := a.execReadOnly(ctx, []string{cmdState, cmdPower})
	if err != nil {
		return nil, err
	}
	onus := ParseONUStateOutput(outputs[cmdState])
	st := &olt.ONUStatus{ONUID: cleanID, Status: "unknown"}
	if len(onus) > 0 {
		*st = onus[0]
	}
	if rx, ok := parseRxPowerDBm(outputs[cmdPower]); ok {
		st.RxPower = rx
	}
	return st, nil
}

// GetSummary queries system identity, card inventory, and ONU counts in one read-only Telnet session.
func (a *ZteC320Adapter) GetSummary(ctx context.Context) (*olt.OLTSummary, error) {
	outputs, err := a.execReadOnly(ctx, []string{"show system-group", "show card", "show gpon onu state"})
	if err != nil {
		return nil, err
	}
	sum := &olt.OLTSummary{
		Vendor:   "ZTE",
		ReadOnly: true,
		PolledAt: time.Now().UTC().Format(time.RFC3339),
	}
	parseSystemGroup(outputs["show system-group"], sum)
	sum.CardsTotal, sum.CardsInService = parseCards(outputs["show card"])

	onus := ParseONUStateOutput(outputs["show gpon onu state"])
	sum.TotalONUs = len(onus)
	for _, o := range onus {
		switch o.Status {
		case "online":
			sum.OnlineONUs++
		case "los":
			sum.LosONUs++
		case "dying_gasp":
			sum.DyingGaspONUs++
		default:
			sum.OfflineONUs++
		}
	}
	return sum, nil
}

// execReadOnly validates all commands against ValidateReadOnlyCommand before dialing, runs them in 1 session, and cleanly exits.
func (a *ZteC320Adapter) execReadOnly(ctx context.Context, cmds []string) (map[string]string, error) {
	if a.host == "" {
		return nil, errors.New("zte_c320: host not configured")
	}
	for _, cmd := range cmds {
		if err := ValidateReadOnlyCommand(cmd); err != nil {
			return nil, err
		}
	}

	addr := net.JoinHostPort(a.host, strconv.Itoa(a.port))
	d := net.Dialer{Timeout: a.timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("zte_c320: dial %s: %w", addr, err)
	}
	defer func() {
		_, _ = conn.Write([]byte("exit\r\n"))
		_ = conn.Close()
	}()

	deadline := time.Now().Add(a.timeout * 2)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)

	if _, err := readUntil(conn, func(s string) bool {
		return reTelnetPrompts.user.MatchString(strings.TrimRight(s, " "))
	}, a.timeout); err != nil {
		return nil, fmt.Errorf("zte_c320: waiting username prompt: %w", err)
	}

	if _, err := conn.Write([]byte(a.user + "\r\n")); err != nil {
		return nil, fmt.Errorf("zte_c320: write username: %w", err)
	}
	if _, err := readUntil(conn, func(s string) bool {
		return reTelnetPrompts.pass.MatchString(strings.TrimRight(s, " "))
	}, a.timeout); err != nil {
		return nil, fmt.Errorf("zte_c320: waiting password prompt: %w", err)
	}

	if _, err := conn.Write([]byte(a.pass + "\r\n")); err != nil {
		return nil, fmt.Errorf("zte_c320: write password: %w", err)
	}
	authOut, err := readUntil(conn, func(s string) bool {
		return rePromptLine.MatchString(s) || reTelnetPrompts.err.MatchString(s)
	}, a.timeout)
	if err != nil {
		return nil, fmt.Errorf("zte_c320: auth timeout: %w", err)
	}
	if reTelnetPrompts.err.MatchString(authOut) {
		return nil, errors.New("zte_c320: authentication failed")
	}

	// Best-effort disable paging using strictly allowed command.
	_, _ = runSingleCmd(conn, "terminal length 0", a.timeout)

	results := make(map[string]string, len(cmds))
	for _, cmd := range cmds {
		out, err := runSingleCmd(conn, cmd, a.timeout)
		if err != nil {
			return nil, fmt.Errorf("zte_c320: command %q failed: %w", cmd, err)
		}
		results[cmd] = out
	}
	return results, nil
}

func runSingleCmd(conn net.Conn, cmd string, stepTimeout time.Duration) (string, error) {
	if _, err := conn.Write([]byte(cmd + "\r\n")); err != nil {
		return "", err
	}
	var all strings.Builder
	for i := 0; i < 64; i++ {
		chunk, err := readUntil(conn, func(s string) bool {
			return rePromptLine.MatchString(s) || strings.Contains(s, "--More--")
		}, stepTimeout)
		all.WriteString(chunk)
		if err != nil {
			return all.String(), err
		}
		if rePromptLine.MatchString(chunk) {
			break
		}
		if strings.Contains(chunk, "--More--") {
			if _, werr := conn.Write([]byte(" ")); werr != nil {
				return all.String(), werr
			}
		}
	}
	return cleanCLIOutput(all.String(), cmd), nil
}

func readUntil(conn net.Conn, match func(string) bool, timeout time.Duration) (string, error) {
	var text bytes.Buffer
	tmp := make([]byte, 4096)
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		n, err := conn.Read(tmp)
		if n > 0 {
			text.Write(stripAndReplyTelnetIAC(conn, tmp[:n]))
			if match(text.String()) {
				return text.String(), nil
			}
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return text.String(), err
		}
	}
	return text.String(), fmt.Errorf("read timeout after %s", timeout)
}

func stripAndReplyTelnetIAC(conn net.Conn, raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if b == 255 && i+1 < len(raw) {
			cmd := raw[i+1]
			if (cmd == 251 || cmd == 252 || cmd == 253 || cmd == 254) && i+2 < len(raw) {
				opt := raw[i+2]
				switch cmd {
				case 253: // DO -> WONT
					_, _ = conn.Write([]byte{255, 252, opt})
				case 251: // WILL -> DONT
					_, _ = conn.Write([]byte{255, 254, opt})
				}
				i += 2
				continue
			}
			if cmd == 250 { // SB ... SE
				end := bytes.Index(raw[i+2:], []byte{255, 240})
				if end >= 0 {
					i = i + 2 + end + 1
					continue
				}
			}
			i++
			continue
		}
		if b == 0 {
			continue
		}
		out = append(out, b)
	}
	return out
}

func cleanCLIOutput(raw, cmd string) string {
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "" || t == cmd || strings.Contains(t, "--More--") {
			continue
		}
		if rePromptLine.MatchString(t) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// ParseONUStateOutput parses ZTE C320 `show gpon onu state` output into canonical ONUStatus records.
func ParseONUStateOutput(raw string) []olt.ONUStatus {
	var out []olt.ONUStatus
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "1/") && !strings.HasPrefix(trimmed, "0/") {
			continue
		}
		m := reONUStateLine.FindStringSubmatch(trimmed)
		if m == nil {
			continue
		}
		out = append(out, olt.ONUStatus{
			ONUID:  m[1] + ":" + m[2],
			Status: mapPhaseState(m[5]),
		})
	}
	return out
}

func mapPhaseState(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "working", "ready", "online":
		return "online"
	case "los", "losi":
		return "los"
	case "dyinggasp", "dying-gasp":
		return "dying_gasp"
	case "offline", "off-line":
		return "offline"
	default:
		return "unknown"
	}
}

func parseRxPowerDBm(raw string) (float64, bool) {
	m := reOpticalValue.FindStringSubmatch(raw)
	if m == nil {
		return 0, false
	}
	val, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

func parseSystemGroup(raw string, sum *olt.OLTSummary) {
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		low := strings.ToLower(t)
		switch {
		case strings.HasPrefix(low, "system description:"):
			desc := strings.TrimSpace(t[len("system description:"):])
			if strings.Contains(desc, "C320") {
				sum.Model = "ZTE C320"
			} else if strings.Contains(desc, "C300") {
				sum.Model = "ZTE C300"
			}
			if idx := strings.Index(desc, "Version "); idx >= 0 {
				rest := strings.TrimSpace(desc[idx+len("Version "):])
				if sp := strings.IndexAny(rest, " ,"); sp > 0 {
					sum.Firmware = rest[:sp]
				} else {
					sum.Firmware = rest
				}
			}
		case strings.HasPrefix(low, "started before:"):
			sum.Uptime = strings.TrimSpace(t[len("started before:"):])
		case strings.HasPrefix(low, "system name:"):
			sum.SystemName = strings.TrimSpace(t[len("system name:"):])
		case strings.HasPrefix(low, "location:"):
			sum.Location = strings.TrimSpace(t[len("location:"):])
		}
	}
}

func parseCards(raw string) (total, inService int) {
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if m := reCardLine.FindStringSubmatch(t); m != nil {
			total++
			if strings.EqualFold(m[1], "INSERVICE") {
				inService++
			}
		}
	}
	return total, inService
}
