package sshclient

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

// ConnectOptions holds everything required to open an SSH connection.
type ConnectOptions struct {
	Name       string
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string // PEM content, optional
	// Proxy settings. ProxyPassword is supplied per-connect and never persisted.
	ProxyType     string // "" | "none" | "socks5" | "http"
	ProxyHost     string
	ProxyPort     int
	ProxyUsername string
	ProxyPassword string
	// ExecCmd, when non-empty, is sent to the remote shell right after connect
	// (and again after each automatic reconnect), e.g. "ls -la".
	ExecCmd string
	// SavedID is the database id of the saved session profile this connection
	// belongs to (used to restore sessions on startup).
	SavedID int64
	// Cols/Rows 是 PTY 的初始终端尺寸(来自 xterm 实际测量, 避免 80 列硬编码
	// 导致 bash PS1 截断/换行错位)。<=0 时回退 80x24。
	Cols int
	Rows int
	// HostKeyCallback 校验服务端主机密钥(known_hosts 思路)。
	// 为 nil 时回退 ssh.InsecureIgnoreHostKey()(正常流程由 app.go 注入真实回调)。
	HostKeyCallback ssh.HostKeyCallback
}

// reconnectInterval is how long to wait before attempting to reconnect after
// the connection drops unexpectedly.
const reconnectInterval = 12 * time.Second

// Session wraps an active SSH client + channels. It survives network drops by
// automatically reconnecting (unless the user closed it manually).
type Session struct {
	ID   string
	Opts ConnectOptions

	mu         sync.Mutex // guards client/sftp/stdin/stdout/stderr/sshSession
	client     *ssh.Client
	sftp       *sftp.Client
	sshSession *ssh.Session
	stdin      io.WriteCloser
	stdout     io.Reader
	stderr     io.Reader

	// emitMu serializes onData callbacks from stdout/stderr pumps. Two
	// concurrent readers may interleave their reads, but the data must reach
	// the terminal in the exact order the remote PTY produced it; otherwise
	// xterm.js can mis-render line-editing sequences (Tab completion echo,
	// cursor moves, etc.) and characters end up at the wrong position.
	emitMu sync.Mutex

	onData  func(id string, data []byte)
	onClose func(id string, reason string)

	stopRead       chan struct{}
	closeStopOnce sync.Once
	closedByUser   bool
	reconnectOnce  sync.Once
	// reconnectNow 用于在用户输入时立即触发一次重连尝试(不等 12s 定时)
	reconnectNow chan struct{}
	wg            sync.WaitGroup

	// netMu 保护网卡速率采样缓存(CollectStats 计算 rx/tx 每秒差值)
	netMu     sync.Mutex
	lastNetAt time.Time
	lastNetRx uint64
	lastNetTx uint64

	// statsMu 保护 lastStats(上次成功采集的完整数据; 某条命令失败时回退用,
	// 避免状态条因个别字段缺失而闪烁)
	statsMu   sync.Mutex
	lastStats *Stats
}

var (
	sessions   = make(map[string]*Session)
	sessionMux sync.RWMutex
)

// Connect establishes an SSH connection and starts IO pumps.
func Connect(opts ConnectOptions, onData func(id string, data []byte), onClose func(id string, reason string)) (*Session, error) {
	if opts.Port == 0 {
		opts.Port = 22
	}

	sess := &Session{
		ID:           fmt.Sprintf("%s_%d", opts.Host, time.Now().UnixNano()),
		Opts:         opts,
		onData:       onData,
		onClose:      onClose,
		stopRead:     make(chan struct{}),
		reconnectNow: make(chan struct{}, 1),
	}

	if err := sess.establish(); err != nil {
		return nil, err
	}

	sessionMux.Lock()
	sessions[sess.ID] = sess
	sessionMux.Unlock()

	sess.startPumps()
	sess.runExecCmd()
	return sess, nil
}

// establish opens a fresh SSH connection + shell for the session, replacing any
// previous connection. Called on first connect and on automatic reconnect.
func (s *Session) establish() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	if s.sftp != nil {
		_ = s.sftp.Close()
		s.sftp = nil
	}
	s.sshSession = nil
	s.stdin, s.stdout, s.stderr = nil, nil, nil

	opts := s.Opts
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))

	auth, err := buildAuth(opts)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	hkc := opts.HostKeyCallback
	if hkc == nil {
		hkc = ssh.InsecureIgnoreHostKey()
	}

	config := &ssh.ClientConfig{
		User:            opts.Username,
		Auth:            auth,
		HostKeyCallback: hkc,
		Timeout:         15 * time.Second,
	}

	conn, err := dialTarget(opts)
	if err != nil {
		return fmt.Errorf("ssh dial: %w", err)
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("ssh handshake: %w", err)
	}
	client := ssh.NewClient(c, chans, reqs)

	sshSession, err := client.NewSession()
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("new session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	// PTY 初始尺寸: 用前端测量的 xterm 列/行, 避免 80 列硬编码导致 PS1 截断
	cols, rows := s.Opts.Cols, s.Opts.Rows
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	// 注意: RequestPty 签名是 (term, h=height/rows, w=width/cols, modes)。
	// 之前把 cols/rows 顺序传反 (cols 当高度、rows 当宽度), 导致服务器 PTY 行列交换,
	// 表现为 stty size 输出 "cols行 rows列"、top 等全屏程序只识别到 50 列宽。
	if err := sshSession.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return fmt.Errorf("request pty: %w", err)
	}

	stdin, err := sshSession.StdinPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := sshSession.StdoutPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := sshSession.StderrPipe()
	if err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := sshSession.Shell(); err != nil {
		_ = sshSession.Close()
		_ = client.Close()
		return fmt.Errorf("start shell: %w", err)
	}

	s.client = client
	s.sshSession = sshSession
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr
	return nil
}

// Resize 动态调整 PTY 行列(前端 xterm fit 后同步, 保证 bash 排版与实际宽度一致)。
func (s *Session) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sshSession == nil {
		return errors.New("session not connected")
	}
	// WindowChange 签名是 (h=height/rows, w=width/cols)。必须传 (rows, cols),
	// 否则与上同因行列交换, 服务器 PTY 宽度被设为前端高度值。
	return s.sshSession.WindowChange(rows, cols)
}

// dialTarget establishes the underlying TCP connection, optionally via a proxy.
func dialTarget(opts ConnectOptions) (net.Conn, error) {
	addr := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	pt := strings.ToLower(opts.ProxyType)
	if pt == "" || pt == "none" {
		return net.DialTimeout("tcp", addr, 15*time.Second)
	}

	proxyAddr := net.JoinHostPort(opts.ProxyHost, strconv.Itoa(opts.ProxyPort))
	auth := (*proxy.Auth)(nil)
	if opts.ProxyUsername != "" {
		auth = &proxy.Auth{User: opts.ProxyUsername, Password: opts.ProxyPassword}
	}

	switch pt {
	case "socks5":
		dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("socks5 proxy: %w", err)
		}
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("socks5 dial: %w", err)
		}
		if err := conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	case "http":
		return dialHTTPProxy(proxyAddr, addr, auth)
	default:
		return nil, fmt.Errorf("unsupported proxy type: %s", opts.ProxyType)
	}
}

// dialHTTPProxy establishes a TCP connection through an HTTP CONNECT proxy.
func dialHTTPProxy(proxyAddr, targetAddr string, auth *proxy.Auth) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect proxy: %w", err)
	}
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)
	if auth != nil && auth.User != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(auth.User + ":" + auth.Password))
		req += "Proxy-Authorization: Basic " + cred + "\r\n"
	}
	req += "\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send connect request: %w", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(15 * time.Second)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read proxy response: %w", err)
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, err
	}

	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "HTTP/1.1 200") && !strings.HasPrefix(resp, "HTTP/1.0 200") {
		_ = conn.Close()
		return nil, fmt.Errorf("proxy connect failed: %s", strings.Split(resp, "\r\n")[0])
	}
	return conn, nil
}

// buildAuth 构造 SSH 认证方法; 既无密码也无私钥时显式报错(不再返回 nil 让 SSH 报笼统错误)。
func buildAuth(opts ConnectOptions) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if opts.Password != "" {
		methods = append(methods, ssh.Password(opts.Password))
	}
	if opts.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(opts.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, errors.New("未配置认证方式(密码或私钥)")
	}
	return methods, nil
}

// Send writes data to the remote shell.
// 若 session 当前未连接(刚断开/正在重连): 主动触发一次立即重连尝试(不等 12s 定时),
// 本次命令不发送, 返回 nil 不报错 — 用户重连后重发即可。
func (s *Session) Send(data []byte) error {
	s.mu.Lock()
	stdin := s.stdin
	s.mu.Unlock()
	if stdin != nil {
		_, err := stdin.Write(data)
		return err
	}
	s.ForceReconnect()
	return nil
}

// ForceReconnect 主动触发一次重连尝试(立即, 不等 12s 定时); 若已关闭或正在重连则 no-op。
func (s *Session) ForceReconnect() {
	s.mu.Lock()
	if s.closedByUser {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.handleDisconnect() // 幂等: reconnectOnce.Do 只跑一次, 启动 reconnectLoop(若未启动)
	select {
	case s.reconnectNow <- struct{}{}:
	default:
	}
}

// GetSFTP returns an SFTP client, lazily initialized.
func (s *Session) GetSFTP() (*sftp.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sftp != nil {
		return s.sftp, nil
	}
	if s.client == nil {
		return nil, fmt.Errorf("session not connected")
	}
	c, err := sftp.NewClient(s.client)
	if err != nil {
		return nil, err
	}
	s.sftp = c
	return c, nil
}

// Close tears down the SSH session and disables auto-reconnect.
func (s *Session) Close() {
	s.mu.Lock()
	s.closedByUser = true
	s.mu.Unlock()

	s.closeStopOnce.Do(func() { close(s.stopRead) })
	s.teardown()
	s.wg.Wait()

	sessionMux.Lock()
	delete(sessions, s.ID)
	sessionMux.Unlock()

	if s.onClose != nil {
		s.onClose(s.ID, "closed")
	}
}

// teardown releases the current connection resources.
func (s *Session) teardown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sftp != nil {
		_ = s.sftp.Close()
		s.sftp = nil
	}
	if s.sshSession != nil {
		_ = s.sshSession.Close()
		s.sshSession = nil
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	s.stdin, s.stdout, s.stderr = nil, nil, nil
}

// startPumps launches stdout/stderr reader goroutines for the current connection.
func (s *Session) startPumps() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdout == nil || s.stderr == nil {
		return
	}
	s.wg.Add(2)
	go s.pump(s.stdout, false)
	go s.pump(s.stderr, true)
}

func (s *Session) pump(r io.Reader, isErr bool) {
	defer s.wg.Done()
	buf := make([]byte, 4096)
	for {
		select {
		case <-s.stopRead:
			return
		default:
		}
		n, err := r.Read(buf)
		if n > 0 && s.onData != nil {
			data := make([]byte, n)
			copy(data, buf[:n])
			// stderr merged into stdout stream for the terminal.
			_ = isErr
			// Serialize across stdout/stderr pumps to preserve PTY byte order.
			s.emitMu.Lock()
			s.onData(s.ID, data)
			s.emitMu.Unlock()
		}
		if err != nil {
			s.handleDisconnect()
			return
		}
	}
}

// handleDisconnect reacts to an unexpected read failure. If the session was
// closed by the user, nothing happens (Close already cleaned up); otherwise a
// single background reconnect loop is started.
func (s *Session) handleDisconnect() {
	s.mu.Lock()
	byUser := s.closedByUser
	s.mu.Unlock()
	if byUser {
		return
	}
	s.reconnectOnce.Do(func() {
		s.teardown()
		s.note("connection lost, reconnecting in 12s...")
		go s.reconnectLoop()
	})
}

// reconnectLoop keeps trying to restore the connection every 12 seconds until
// it succeeds or the session is closed by the user.
// 可通过 reconnectNow channel 被立即唤醒(用户输入时不等 12s 定时)。
func (s *Session) reconnectLoop() {
	for {
		select {
		case <-s.stopRead:
			return
		case <-s.reconnectNow:
		case <-time.After(reconnectInterval):
		}

		s.mu.Lock()
		byUser := s.closedByUser
		s.mu.Unlock()
		if byUser {
			return
		}

		if err := s.establish(); err != nil {
			s.note(fmt.Sprintf("reconnect failed (%v), retrying...", err))
			continue
		}

		// Stop immediately if the user closed the tab while we were dialing.
		select {
		case <-s.stopRead:
			s.teardown()
			return
		default:
		}

		s.note("reconnected.")
		s.startPumps()
		s.runExecCmd()
		return
	}
}

// note writes a status message into the terminal data stream.
func (s *Session) note(msg string) {
	if s.onData != nil {
		s.onData(s.ID, []byte("\r\n["+msg+"]\r\n"))
	}
}

// runExecCmd sends the configured startup command to the remote shell.
func (s *Session) runExecCmd() {
	if s.Opts.ExecCmd == "" {
		return
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = s.Send([]byte(s.Opts.ExecCmd + "\r"))
	}()
}

// Find returns an active session by id.
func Find(id string) *Session {
	sessionMux.RLock()
	defer sessionMux.RUnlock()
	return sessions[id]
}

// List returns all active session ids.
func List() []string {
	sessionMux.RLock()
	defer sessionMux.RUnlock()
	ids := make([]string, 0, len(sessions))
	for id := range sessions {
		ids = append(ids, id)
	}
	return ids
}

// Duplicate creates a new active session using the same connection options.
func Duplicate(id string, onData func(id string, data []byte), onClose func(id string, reason string)) (*Session, error) {
	sessionMux.RLock()
	s := sessions[id]
	sessionMux.RUnlock()
	if s == nil {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return Connect(s.Opts, onData, onClose)
}

// ===========================================================================
// 服务器状态采集(Stats)
// ===========================================================================

// Stats 服务器实时状态(由前端 SessionStatsBar 展示)。
// 字段缺失表示采集失败(0 不一定代表 0, 也可能表示该指标未取到; 前端按需展示)。
type Stats struct {
	Host        string  `json:"host"`
	CpuPercent  float64 `json:"cpuPercent"`  // CPU 使用率 0-100
	MemTotal    uint64  `json:"memTotal"`    // 内存总 (MB)
	MemUsed     uint64  `json:"memUsed"`     // 内存已用 (MB)
	MemPercent  float64 `json:"memPercent"`  // 内存使用率 0-100
	DiskTotal   uint64  `json:"diskTotal"`   // / 盘总 (bytes)
	DiskUsed    uint64  `json:"diskUsed"`    // / 盘已用 (bytes)
	DiskPercent float64 `json:"diskPercent"` // 磁盘使用率 0-100
	CpuCores    int     `json:"cpuCores"`    // 物理核心数
	CpuThreads  int     `json:"cpuThreads"`  // 逻辑线程数 (nproc)
	ProcCount   int     `json:"procCount"`   // 进程数 (/proc/[0-9]* 数量)
	LoadAvg     string  `json:"loadAvg"`     // 1/5/15 分钟负载
	Uptime      string  `json:"uptime"`      // 已运行时长(中文格式: X天X小时X分X秒)
	NetRx       uint64  `json:"netRx"`       // 网卡下行速率 bytes/s(两次采样差值)
	NetTx       uint64  `json:"netTx"`       // 网卡上行速率 bytes/s
	ConnCount   int     `json:"connCount"`   // ESTABLISHED TCP 连接数
	SwapTotal   uint64  `json:"swapTotal"`   // Swap 总 (MB)
	SwapUsed    uint64  `json:"swapUsed"`    // Swap 已用 (MB)
	SwapPercent float64 `json:"swapPercent"` // Swap 使用率 0-100
	IoWait      float64 `json:"ioWait"`      // IO 等待占比 %
	Kernel      string  `json:"kernel"`      // 系统版本(发行版名, 退化内核版本)
	Boottime    int64   `json:"boottime"`    // 系统启动时间(Unix 秒); 前端每秒本地计算 uptime
	FetchedAt   int64   `json:"fetchedAt"`   // 采集时间(unix 秒)
}

// CollectStats 通过并发 exec channel 采集服务器状态。
// 命令不通过交互 PTY 发送, 不会污染终端显示。
func (s *Session) CollectStats() (*Stats, error) {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		return nil, errors.New("session not connected")
	}

	stats := &Stats{Host: s.Opts.Host, FetchedAt: time.Now().Unix()}
	// prev 为上次成功采集的数据; 某条命令输出为空(采集失败)时回退到 prev 的对应字段,
	// 避免状态条因个别字段缺失而闪烁(注意: 判据是"输出是否为空", 合法 0 不受影响)
	s.statsMu.Lock()
	prev := s.lastStats
	s.statsMu.Unlock()

	// 并发跑 11 个远程命令: 每个写独立变量, 无 race; 失败的命令容错(对应字段保持 0)
	var cpuOut, memOut, diskOut, coreOut, threadOut, procOut, loadOut, uptimeOut, netOut, connOut, sysOut, btimeOut string
	var wg sync.WaitGroup
	wg.Add(12)
	go func() {
		defer wg.Done()
		// 两次采样 /proc/stat 的 cpu 聚合行, 间隔 1s, 算差值得到真实瞬时 CPU 使用率与 IO 等待;
		// 比解析 top -bn1 文本更稳定(不依赖 top 版本/语言, 且避开 top -bn1 首帧采样偏斜)。
		cpuOut, _ = runRemoteCmd(client, `grep '^cpu ' /proc/stat; sleep 1; grep '^cpu ' /proc/stat`)
	}()
	go func() {
		defer wg.Done()
		// /proc/meminfo 兼容所有 Linux(busybox/free 不可用时也能采集), 字段单位 kB → MB
		memOut, _ = runRemoteCmd(client, `grep -E '^(MemTotal|MemAvailable|SwapTotal|SwapFree):' /proc/meminfo`)
	}()
	go func() {
		defer wg.Done()
		diskOut, _ = runRemoteCmd(client, `df -B1 / | tail -1`)
	}()
	go func() {
		defer wg.Done()
		// 物理核数: 每个 processor 条目的 cpu cores(每 socket), 乘 socket 数
		coreOut, _ = runRemoteCmd(client, `awk '/^cpu cores/{c=$4} /^physical id/{if(!($3 in s))s[$3]=1} END{n=0; for(k in s)n++; print (c>0 ? c*(n>0?n:1) : "0")}' /proc/cpuinfo`)
	}()
	go func() {
		defer wg.Done()
		// 逻辑线程数: nproc; 退化 /proc/cpuinfo 的 processor 数
		out, err := runRemoteCmd(client, `nproc 2>/dev/null`)
		if err != nil || strings.TrimSpace(out) == "" {
			out, _ = runRemoteCmd(client, `grep -c '^processor' /proc/cpuinfo`)
		}
		threadOut = out
	}()
	go func() {
		defer wg.Done()
		procOut, _ = runRemoteCmd(client, `ls -d /proc/[0-9]* 2>/dev/null | wc -l`)
	}()
	go func() {
		defer wg.Done()
		loadOut, _ = runRemoteCmd(client, `cat /proc/loadavg`)
	}()
	go func() {
		defer wg.Done()
		// 中文精确到秒: X天X小时X分X秒
		uptimeOut, _ = runRemoteCmd(client, `awk '{d=int($1/86400);h=int(($1%86400)/3600);m=int(($1%3600)/60);s=int($1%60);printf "%d天%d小时%d分%d秒",d,h,m,s}' /proc/uptime`)
	}()
	go func() {
		defer wg.Done()
		// 聚合所有非 lo 接口的 rx/tx bytes
		netOut, _ = runRemoteCmd(client, `awk 'NR>2 && $1!="lo:" {rx+=$2; tx+=$10} END {print rx+0, tx+0}' /proc/net/dev`)
	}()
	go func() {
		defer wg.Done()
		// ESTABLISHED TCP 连接数(tcp/tcp6 状态列 01)
		connOut, _ = runRemoteCmd(client, `cat /proc/net/tcp /proc/net/tcp6 2>/dev/null | awk '$4=="01"{c++} END{print c+0}'`)
	}()
	go func() {
		defer wg.Done()
		// 系统版本: 优先发行版名, 退化内核版本
		sysOut, _ = runRemoteCmd(client, `grep '^PRETTY_NAME=' /etc/os-release 2>/dev/null | cut -d'=' -f2 | tr -d '"' || uname -r`)
	}()
	go func() {
		defer wg.Done()
		// 启动时间 (Unix 秒): /proc/stat 的 btime 字段; 前端每秒本地计算 uptime
		btimeOut, _ = runRemoteCmd(client, `awk '/^btime /{print $2; exit}' /proc/stat`)
	}()
	wg.Wait()

	if strings.TrimSpace(cpuOut) != "" {
		cpuPct, ioWaitPct := parseProcStatCPU(cpuOut)
		stats.CpuPercent = cpuPct
		stats.IoWait = ioWaitPct
	} else if prev != nil {
		stats.CpuPercent = prev.CpuPercent
		stats.IoWait = prev.IoWait
	}
	if strings.TrimSpace(memOut) != "" {
		memTotal, memUsed, memPct, swapTotal, swapUsed, swapPct := parseMemInfo(memOut)
		stats.MemTotal = uint64(memTotal)
		stats.MemUsed = uint64(memUsed)
		stats.MemPercent = memPct
		stats.SwapTotal = uint64(swapTotal)
		stats.SwapUsed = uint64(swapUsed)
		stats.SwapPercent = swapPct
	} else if prev != nil {
		stats.MemTotal = prev.MemTotal
		stats.MemUsed = prev.MemUsed
		stats.MemPercent = prev.MemPercent
		stats.SwapTotal = prev.SwapTotal
		stats.SwapUsed = prev.SwapUsed
		stats.SwapPercent = prev.SwapPercent
	}
	if strings.TrimSpace(diskOut) != "" {
		stats.DiskTotal, stats.DiskUsed, stats.DiskPercent = parseDFBytes(diskOut)
	} else if prev != nil {
		stats.DiskTotal = prev.DiskTotal
		stats.DiskUsed = prev.DiskUsed
		stats.DiskPercent = prev.DiskPercent
	}
	if strings.TrimSpace(coreOut) != "" {
		fmt.Sscanf(strings.TrimSpace(coreOut), "%d", &stats.CpuCores)
	} else if prev != nil {
		stats.CpuCores = prev.CpuCores
	}
	if strings.TrimSpace(threadOut) != "" {
		fmt.Sscanf(strings.TrimSpace(threadOut), "%d", &stats.CpuThreads)
	} else if prev != nil {
		stats.CpuThreads = prev.CpuThreads
	}
	if strings.TrimSpace(procOut) != "" {
		fmt.Sscanf(strings.TrimSpace(procOut), "%d", &stats.ProcCount)
	} else if prev != nil {
		stats.ProcCount = prev.ProcCount
	}
	parts := strings.Fields(loadOut)
	if len(parts) >= 3 {
		stats.LoadAvg = strings.Join(parts[:3], " ")
	} else if prev != nil {
		stats.LoadAvg = prev.LoadAvg
	}
	if u := strings.TrimSpace(uptimeOut); u != "" {
		stats.Uptime = u
	} else if prev != nil {
		stats.Uptime = prev.Uptime
	}
	if k := strings.TrimSpace(sysOut); k != "" {
		stats.Kernel = k
	} else if prev != nil {
		stats.Kernel = prev.Kernel
	}
	if b := strings.TrimSpace(btimeOut); b != "" {
		if v, err := strconv.ParseInt(b, 10, 64); err == nil {
			stats.Boottime = v
		}
	} else if prev != nil {
		stats.Boottime = prev.Boottime
	}
	if strings.TrimSpace(connOut) != "" {
		stats.ConnCount = parseConnCount(connOut)
	} else if prev != nil {
		stats.ConnCount = prev.ConnCount
	}
	// 网卡速率: 与上次采样做差值(首次采样返回 0, 下一次刷新才有值);
	// 输出为空(采集失败)时不上滑采样点, 并回退到上次速率, 避免速率跳变/闪烁
	rx, tx := parseNetDev(netOut)
	s.netMu.Lock()
	now := time.Now()
	netOK := false
	if strings.TrimSpace(netOut) != "" {
		if !s.lastNetAt.IsZero() {
			dt := now.Sub(s.lastNetAt).Seconds()
			if dt > 0 && rx >= s.lastNetRx && tx >= s.lastNetTx {
				stats.NetRx = uint64(float64(rx-s.lastNetRx) / dt)
				stats.NetTx = uint64(float64(tx-s.lastNetTx) / dt)
				netOK = true
			}
		}
		s.lastNetAt = now
		s.lastNetRx = rx
		s.lastNetTx = tx
	}
	if !netOK && prev != nil {
		stats.NetRx = prev.NetRx
		stats.NetTx = prev.NetTx
	}
	s.netMu.Unlock()

	// 保存本次结果供下次回退
	s.statsMu.Lock()
	s.lastStats = stats
	s.statsMu.Unlock()
	return stats, nil
}

// runRemoteCmd 通过独立 exec channel 一次性执行一条命令(不占用交互 PTY),
// 输出经 string 返回。命令失败/超时时返回错误但不阻塞其他命令。
func runRemoteCmd(client *ssh.Client, cmd string) (string, error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.Output(cmd)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// parseProcStatCPU 从两次 /proc/stat 的 cpu 聚合行(中间已 sleep 1s)算差值,
// 返回 (CPU 使用率%, IO 等待%)。字段顺序: user nice system idle iowait irq softirq steal guest guest_nice。
// guest(8)/guest_nice(9) 已计入 user(0)/nice(1), 不重复计入 total(同 procps 算法), 避免重复统计虚拟化开销。
// 输入示例:
//   cpu  702630 0 2595466 3551563 1187 0 28574 0 0 0
//   cpu  702635 0 2595471 3551568 1187 0 28574 0 0 0
func parseProcStatCPU(s string) (float64, float64) {
	var samples []string
	for _, l := range strings.Split(s, "\n") {
		if strings.HasPrefix(l, "cpu ") { // 仅聚合行, 排除 cpu0/cpu1...
			samples = append(samples, l)
		}
	}
	if len(samples) < 2 {
		return 0, 0
	}
	a := parseCpuFields(samples[0])
	b := parseCpuFields(samples[1])
	if a == nil || b == nil {
		return 0, 0
	}
	d := make([]float64, 10)
	for i := 0; i < 10; i++ {
		d[i] = b[i] - a[i]
	}
	// 仅前 8 项计入 total(guest/guest_nice 已含于 user/nice)
	total := d[0] + d[1] + d[2] + d[3] + d[4] + d[5] + d[6] + d[7]
	if total <= 0 {
		return 0, 0
	}
	idleTotal := d[3] + d[4] // idle + iowait
	used := total - idleTotal
	cpuPct := used / total * 100
	ioWaitPct := d[4] / total * 100
	return math.Round(cpuPct*10) / 10, math.Round(ioWaitPct*10) / 10
}

// parseCpuFields 解析 /proc/stat 的 cpu 行, 返回前 10 个数值字段(不足返回 nil)。
func parseCpuFields(line string) []float64 {
	f := strings.Fields(line)
	if len(f) < 5 { // "cpu" + 至少 user/nice/system/idle
		return nil
	}
	vals := make([]float64, 10)
	n := 0
	for i := 1; i < len(f) && n < 10; i++ {
		v, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return nil
		}
		vals[n] = v
		n++
	}
	if n < 4 {
		return nil
	}
	return vals
}

// parseDFBytes 解析 df -B1 / | tail -1, 返回 total/used(bytes) 和使用率。
// 输入示例: "/dev/sda1 50000000000 30000000000 20000000000 60% /"
func parseDFBytes(s string) (uint64, uint64, float64) {
	fields := strings.Fields(s)
	if len(fields) < 4 {
		return 0, 0, 0
	}
	total, _ := strconv.ParseUint(fields[1], 10, 64)
	used, _ := strconv.ParseUint(fields[2], 10, 64)
	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}
	return total, used, math.Round(pct*10) / 10
}

// parseConnCount 解析连接数命令输出(纯数字)。
func parseConnCount(s string) int {
	n := 0
	fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n
}

// parseNetDev 解析 /proc/net/dev 聚合输出 "rx tx"(bytes), 已排除 lo。
func parseNetDev(s string) (uint64, uint64) {
	var rx, tx uint64
	fmt.Sscanf(strings.TrimSpace(s), "%d %d", &rx, &tx)
	return rx, tx
}

// parseMemInfo 解析 /proc/meminfo 的 MemTotal/MemAvailable/SwapTotal/SwapFree 行。
// 单位 kB → MB; used = total - available(包含可回收缓存, 更真实)。
// 输入示例:
//   MemTotal:        3880220 kB
//   MemAvailable:    2345678 kB
//   SwapTotal:       1024000 kB
//   SwapFree:         900000 kB
func parseMemInfo(s string) (memTotal, memUsed, memPct, swapTotal, swapUsed, swapPct float64) {
	var memAvail, swapFree float64
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseFloat(fields[1], 64)
		vBytes := v * 1024 // kB → bytes(与磁盘字段统一, 前端 fmtCompact 直接用)
		switch fields[0] {
		case "MemTotal:":
			memTotal = vBytes
		case "MemAvailable:":
			memAvail = vBytes
		case "SwapTotal:":
			swapTotal = vBytes
		case "SwapFree:":
			swapFree = vBytes
		}
	}
	if memTotal > 0 && memAvail > 0 {
		memUsed = memTotal - memAvail
		memPct = math.Round(memUsed/memTotal*100*10) / 10
	}
	if swapTotal > 0 && swapFree > 0 {
		swapUsed = swapTotal - swapFree
		swapPct = math.Round(swapUsed/swapTotal*100*10) / 10
	}
	return
}
