package session

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The control socket is how the editor talks to the game. Each game process
// listens on a unix socket of its own, RunDir/<pid>.sock, and the editor
// reaches it with `katharsis ctl <cmd> [args]`, which finds it through
// $KATHARSIS_SOCK. The protocol is JSON lines: one Command per line in, one
// Reply per line back, in order.
//
//	→ {"cmd":"debug","args":["3"]}
//	← {"ok":true,"text":"lldb on #3: type run"}

// sockEnv carries the socket path to the editor and to `katharsis ctl`.
const sockEnv = "KATHARSIS_SOCK"

// Commands the game answers. Anything else is refused by the server without
// reaching the game.
const (
	CmdSubmit  = "submit"  // judge the full tests
	CmdDebug   = "debug"   // [test]: open lldb on a test
	CmdQuack   = "quack"   // use a rubber duck
	CmdGiveUp  = "giveup"  // :q!, after vim asked
	CmdRetreat = "retreat" // swap the door for a life
	CmdStatus  = "status"  // one line: timer, lives, tests
)

var commands = []string{CmdSubmit, CmdDebug, CmdQuack, CmdGiveUp, CmdRetreat, CmdStatus}

type Command struct {
	Cmd  string   `json:"cmd"`
	Args []string `json:"args,omitempty"`
}

// Reply is the game's answer, one line for vim's message area.
type Reply struct {
	OK   bool   `json:"ok"`
	Text string `json:"text"`
}

// Request is a Command waiting for the game's answer. The server delivers it
// to the HUD as a tea.Msg; the HUD answers once it knows (a submit answers
// when judging ends).
type Request struct {
	Command
	reply chan Reply
}

// NewRequest makes a Request outside the server, for tests and for callers
// that want to route a command (a key press, say) the way the editor would.
func NewRequest(c Command) (Request, <-chan Reply) {
	ch := make(chan Reply, 1)
	return Request{Command: c, reply: ch}, ch
}

// Answer replies to the request. Only the first answer counts, and Answer
// never blocks, so it is safe to call from Update.
func (r Request) Answer(rep Reply) {
	select {
	case r.reply <- rep:
	default:
	}
}

// RunDir is where a game process keeps its files: the socket and the vim
// overlay, named by pid. It is private to the user.
func RunDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "katharsis", "run")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, os.Chmod(dir, 0o700)
}

// SocketPath is this process's control socket in RunDir.
func SocketPath() (string, error) { return runFile(".sock") }

// OverlayPath is this process's vim overlay in RunDir.
func OverlayPath() (string, error) { return runFile(".vim") }

func runFile(ext string) (string, error) {
	dir, err := RunDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, strconv.Itoa(os.Getpid())+ext), nil
}

// SweepRunDir removes files in RunDir left by game processes that no longer
// exist (killed before they could clean up).
func SweepRunDir() error {
	dir, err := RunDir()
	if err != nil {
		return err
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		stem, _, _ := strings.Cut(e.Name(), ".")
		stem, _, _ = strings.Cut(stem, "-")
		pid, err := strconv.Atoi(stem)
		if err != nil || pid == os.Getpid() {
			continue
		}
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

// maxSockPath is sun_path's size on macOS (104, with the NUL); Linux allows
// 108.
const maxSockPath = 104

// Server is a listening control socket.
type Server struct {
	ln *net.UnixListener

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup

	mu     sync.Mutex
	closed bool // under mu, so no handler is added once Close waits
	conns  map[net.Conn]struct{}
}

// Listen opens the control socket at path, readable by this user only. A
// socket file already there is replaced if nothing answers on it (a game
// that died with this pid); a live one is an error.
func Listen(path string) (*Server, error) {
	if len(path) >= maxSockPath {
		return nil, fmt.Errorf("session: socket path is %d bytes, over the %d the OS allows: %s",
			len(path), maxSockPath-1, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); err == nil {
		if c, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
			_ = c.Close()
			return nil, fmt.Errorf("session: another game is listening on %s", path)
		}
		_ = os.Remove(path) // if this fails, so does the listen below
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, fmt.Errorf("session: listen: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	return &Server{ln: ln, done: make(chan struct{}), conns: map[net.Conn]struct{}{}}, nil
}

// Serve accepts connections until Close, hands each command to deliver as
// a Request, and writes back its answer. Each connection has its own
// goroutine, so deliver may block for a while (tea.Program.Send does until
// the program runs), but it must return: Close waits for it. Serve returns
// nil after Close.
func (s *Server) Serve(deliver func(Request)) error {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.done:
				return nil
			default:
				return fmt.Errorf("session: accept: %w", err)
			}
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = c.Close()
			return nil
		}
		s.conns[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go func() {
			defer s.wg.Done()
			s.handle(c, deliver)
			s.mu.Lock()
			delete(s.conns, c)
			s.mu.Unlock()
			_ = c.Close()
		}()
	}
}

func (s *Server) handle(c net.Conn, deliver func(Request)) {
	sc := bufio.NewScanner(c)
	enc := json.NewEncoder(c)
	for sc.Scan() {
		var cmd Command
		var rep Reply
		switch err := json.Unmarshal(sc.Bytes(), &cmd); {
		case err != nil:
			rep.Text = "bad request: " + err.Error()
		case !slices.Contains(commands, cmd.Cmd):
			rep.Text = fmt.Sprintf("unknown command %q (have %s)", cmd.Cmd, strings.Join(commands, ", "))
		default:
			req, ch := NewRequest(cmd)
			deliver(req)
			select {
			case rep = <-ch:
			case <-s.done:
				rep.Text = "the game is shutting down"
			}
		}
		if enc.Encode(rep) != nil {
			return
		}
	}
}

// Close stops the server, ends open connections and removes the socket.
// Requests still waiting are answered "shutting down".
func (s *Server) Close() error {
	var err error
	s.once.Do(func() {
		close(s.done)
		err = s.ln.Close() // unlinks the socket file
		s.mu.Lock()
		s.closed = true
		for c := range s.conns {
			// Not c.Close: that races a waiting request's "shutting down"
			// answer out of the socket. Ending reads stops the handler
			// once it has written; the write deadline covers a client that
			// stopped reading.
			_ = c.SetReadDeadline(time.Now())
			_ = c.SetWriteDeadline(time.Now().Add(time.Second))
		}
		s.mu.Unlock()
		s.wg.Wait()
	})
	return err
}

// call sends one command to the game at path and waits up to timeout for
// the answer.
func call(path string, c Command, timeout time.Duration) (Reply, error) {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		return Reply{}, fmt.Errorf("no game at %s: %w", path, err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return Reply{}, err
	}
	if err := json.NewEncoder(conn).Encode(c); err != nil {
		return Reply{}, err
	}
	var rep Reply
	if err := json.NewDecoder(conn).Decode(&rep); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return Reply{}, fmt.Errorf("no answer to %s within %v", c.Cmd, timeout)
		}
		return Reply{}, fmt.Errorf("reading the answer to %s: %w", c.Cmd, err)
	}
	return rep, nil
}

// callTimeout is how long `katharsis ctl` waits for cmd. A submit answers
// when the judge finishes, which a slow solution on many tests can stretch
// to a minute or more; the editor runs it as a job, so waiting costs
// nothing.
func callTimeout(cmd string) time.Duration {
	switch cmd {
	case CmdSubmit, CmdDebug:
		return 10 * time.Minute
	}
	return 10 * time.Second
}

// CtlMain is `katharsis ctl <cmd> [args]`: it sends one command to the game
// whose socket $KATHARSIS_SOCK names and prints the answer on stdout. The
// exit status is 0 when the game said OK, 1 when it refused, 2 when it could
// not be asked, with the reason on stderr.
func CtlMain(args []string, stdout, stderr io.Writer) int {
	text, code := ctl(args)
	w := stdout
	if code == 2 {
		w = stderr
	}
	_, _ = fmt.Fprintln(w, text)
	return code
}

func ctl(args []string) (text string, code int) {
	if len(args) == 0 {
		return fmt.Sprintf("usage: katharsis ctl <%s> [args]", strings.Join(commands, "|")), 2
	}
	path := os.Getenv(sockEnv)
	if path == "" {
		return fmt.Sprintf("katharsis ctl: $%s is not set; run this from the game's editor", sockEnv), 2
	}
	c := Command{Cmd: args[0], Args: args[1:]}
	rep, err := call(path, c, callTimeout(c.Cmd))
	switch {
	case err != nil:
		return "katharsis ctl: " + err.Error(), 2
	case !rep.OK:
		return rep.Text, 1
	}
	return rep.Text, 0
}
