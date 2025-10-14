package utils

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gliderlabs/ssh"
)

type Message struct {
	Timestamp time.Time
	Username  string
	Content   string
}

type Input struct {
	Buffer []rune
	MaxLen int
}

type Client struct {
	session ssh.Session

	mu       sync.Mutex
	width    int
	height   int
	input    Input
	messages []Message

	wg       sync.WaitGroup
	username string
	ip       string

	// Event channels
	RenderCh         chan struct{}
	EnterCh          chan struct{}
	WinSizeChangedCh chan struct{}
	CloseCh          chan struct{}

	// Debounce
	renderDebounceTimer *time.Timer
	renderDebounceDur   time.Duration

	// internals
	cancel context.CancelFunc
	once   sync.Once
}

// NewClient creates a Client bound to an ssh.Session and initial state.
// It starts background goroutines to watch input, window-size changes, and session close.
func NewClient(s ssh.Session, w int, h int, username string, ip string) *Client {
	input := Input{
		Buffer: make([]rune, 0, 128),
		MaxLen: 128,
	}
	c := &Client{
		session:           s,
		width:             w,
		height:            h,
		username:          username,
		ip:                ip,
		input:             input,
		messages:          make([]Message, 0),
		RenderCh:          make(chan struct{}, 1),
		EnterCh:           make(chan struct{}, 1),
		WinSizeChangedCh:  make(chan struct{}, 1),
		CloseCh:           make(chan struct{}, 1),
		renderDebounceDur: 50 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	// Input watcher (Enter, Ctrl+C, Ctrl+D) and render trigger
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		reader := bufio.NewReader(s)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			r, size, err := reader.ReadRune()
			if err != nil {
				if err == io.EOF || isSessionClosedErr(err) {
					c.emitClose()
				}
				return
			}

			if size == 0 {
				continue
			}

			c.mu.Lock()
			switch r {
			case '\r', '\n': // **[수정] \r과 \n을 함께 처리**
				if len(c.input.Buffer) > 0 {
					c.messages = append(c.messages, Message{
						Timestamp: time.Now(),
						Username:  c.username,
						Content:   string(c.input.Buffer),
					})
					c.input.Buffer = c.input.Buffer[:0]
				}
				c.mu.Unlock()
				c.trySend(c.EnterCh)
				c.TrySendRender()
			case 0x03: // Ctrl+C
				c.mu.Unlock()
				c.emitClose()
				return
			case 0x04: // Ctrl+D
				c.mu.Unlock()
				c.emitClose()
				return
			case '\b', 0x7f: // Backspace (0x08) 또는 Delete (0x7f)
				if len(c.input.Buffer) > 0 {
					c.input.Buffer = c.input.Buffer[:len(c.input.Buffer)-1]
					c.mu.Unlock()
					c.TrySendRender()
				} else {
					c.mu.Unlock()
				}
			default:
				// 출력 불가능한 문자 (제어 문자 등)는 무시
				if r < 0x20 && r != '\t' {
					c.mu.Unlock()
					continue
				}

				if len(c.input.Buffer) < c.input.MaxLen {
					// 룬(rune)을 []rune 슬라이스에 추가
					c.input.Buffer = append(c.input.Buffer, r)
					c.mu.Unlock()
					c.TrySendRender()
				} else {
					// 버퍼가 꽉 찬 경우 렌더링 요청을 보내지 않습니다.
					c.mu.Unlock()
				}
			}
		}
	}()

	// Window size change watcher
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		_, winCh, _ := s.Pty()

		for {
			select {
			case <-ctx.Done():
				return
			case win, ok := <-winCh:
				if !ok {
					return
				}
				c.mu.Lock()
				c.width = win.Width
				c.height = win.Height
				c.mu.Unlock()
				c.trySend(c.WinSizeChangedCh)
				c.TrySendRender()
			}
		}
	}()

	// Session close watcher (fallback)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		<-s.Context().Done()
		c.emitClose()
	}()

	return c
}

// Word wrap을 위한 헬퍼 함수
// 반환되는 각 문자열은 한 줄의 내용이며, 줄 바꿈 문자는 포함하지 않습니다.
func calculateMessageLines(header string, content []rune, w int) []string {
	lines := make([]string, 0)
	currentLine := ""
	lineWidth := w

	// 1. 헤더 처리
	if utf8.RuneCountInString(header) > 0 {
		currentLine = header
		lineWidth -= utf8.RuneCountInString(header)
	}

	// 2. 내용 처리
	contentIdx := 0
	for contentIdx < len(content) {
		remainingContent := content[contentIdx:]
		spaceLeft := lineWidth - utf8.RuneCountInString(currentLine)

		if len(remainingContent) <= spaceLeft {
			// 남은 내용이 현재 줄에 모두 들어갈 경우
			currentLine += string(remainingContent)
			contentIdx = len(content)
		} else if spaceLeft > 0 {
			// 현재 줄에 일부만 넣을 경우
			toAdd := remainingContent[:spaceLeft]
			currentLine += string(toAdd)
			contentIdx += len(toAdd)

			// 줄이 꽉 찼으므로 다음 줄로 넘깁니다.
			lines = append(lines, currentLine)
			currentLine = ""
			lineWidth = w // 다음 줄은 전체 너비
		} else {
			// 현재 줄이 꽉 찼거나 (spaceLeft <= 0), 헤더만 있는 경우
			// 현재 줄을 저장하고 다음 줄로 넘깁니다.
			lines = append(lines, currentLine)
			currentLine = ""
			lineWidth = w // 다음 줄은 전체 너비
		}
	}

	// 마지막 줄이 비어있지 않으면 추가
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// handleRender는 화면 렌더링을 처리합니다.
func (c *Client) handleRender() {
	w, h := c.Size()
	s := c.Session()

	// 1. 화면을 지우고 커서를 맨 위로 이동 (화면을 새로 그릴 준비)
	fmt.Fprint(s, "\x1b[2J\x1b[H")

	// **[수정] 2. 입력 프롬프트 렌더링 영역 계산**
	promptLine := h // 프롬프트가 위치할 맨 아래 행

	// **[수정] 3. 메시지 렌더링 (아래에서 위로 스크롤)**
	maxMessageHeight := promptLine - 1 // 메시지가 출력될 수 있는 최대 행

	messages := c.messages
	currentY := maxMessageHeight // 현재 출력할 행 (bottom-up)

	// 메시지 인덱스를 역순으로 순회 (최신 메시지가 화면 아래에 위치)
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		header := fmt.Sprintf("[%s %s] ", msg.Timestamp.Format("2006-01-02 15:04:05"), msg.Username)
		content := []rune(msg.Content)

		lines := calculateMessageLines(header, content, w)
		numLines := len(lines)

		// 현재 행이 출력될 공간이 부족하면 중단
		if currentY-numLines < 0 {
			break
		}

		// 커서를 메시지가 시작될 행으로 이동 (currentY + 1은 1-based index)
		currentY -= numLines
		fmt.Fprintf(s, "\x1b[%dH", currentY+1)

		// 메시지 출력 (줄 바꿈 포함)
		for _, line := range lines {
			fmt.Fprintln(s, line)
		}
		if currentY < 0 {
			break
		}
	}

	// 4. 입력 프롬프트 렌더링 (화면 맨 아래 행에 출력)
	fmt.Fprintf(s, "\x1b[%dH", promptLine)
	promptRunes := append([]rune("> "), c.input.Buffer...)

	// 프롬프트를 화면 너비 w에 맞게 출력 (줄 바꿈은 고려하지 않음)
	if len(promptRunes) > w {
		fmt.Fprint(s, string(promptRunes[:w]))
	} else {
		fmt.Fprint(s, string(promptRunes))
	}

	// 5. 마지막으로 커서를 입력 위치로 재배치 (promptLine 행, 프롬프트 문자열 끝)
	cursorX := utf8.RuneCountInString("> ") + len(c.input.Buffer) + 1
	if cursorX > w {
		cursorX = w
	}
	fmt.Fprintf(s, "\x1b[%d;%dH", promptLine, cursorX)
}

func (c *Client) handleClose() {
	c.emitClose()
}

// Accessors

func (c *Client) Session() ssh.Session { return c.session }

func (c *Client) Username() string { return c.username }

func (c *Client) IP() string { return c.ip }

func (c *Client) Size() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.width, c.height
}

// Close shuts down watchers and closes event channels.
func (c *Client) Close() {
	c.emitClose()
	c.wg.Wait()
	c.once.Do(func() {
		close(c.RenderCh)
		close(c.EnterCh)
		close(c.WinSizeChangedCh)
		close(c.CloseCh)
	})
}

// Internal utilities
func (c *Client) emitClose() {
	c.once.Do(func() {
		// **[추가]** SSH 세션 자체를 닫습니다.
		if c.session != nil {
			_ = c.session.Close() // 오류 처리는 간단히 무시합니다.
		}

		// drain cancel after a short delay to allow pending events
		go func() {
			time.Sleep(10 * time.Millisecond)
			if c.cancel != nil {
				c.cancel()
			}
		}()
		c.trySend(c.CloseCh)
	})
}

func (c *Client) trySend(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (c *Client) TrySendRender() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.renderDebounceTimer != nil {
		c.renderDebounceTimer.Reset(c.renderDebounceDur)
		return
	}

	c.renderDebounceTimer = time.AfterFunc(c.renderDebounceDur, func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		c.trySend(c.RenderCh)

		c.renderDebounceTimer.Stop()
		c.renderDebounceTimer = nil
	})
}

func (c *Client) HandleRender() {
	c.handleRender()
}

func (c *Client) EventLoop() {
	for {
		select {
		case <-c.RenderCh:
			c.HandleRender()

		case <-c.EnterCh:
			// Input watcher가 메시지 버퍼를 변경하고 EnterCh를 보냈습니다.
			// 상태가 변경되었으므로 렌더링을 요청합니다.
			c.HandleRender()

		case <-c.WinSizeChangedCh:
			// WinSize watcher가 이미 c.width, c.height를 업데이트했습니다.
			c.HandleRender()

		case <-c.CloseCh:
			c.handleClose()
			return
		}
	}
}

func isSessionClosedErr(err error) bool {
	// Conservative check; specific implementations may vary.
	return err == io.EOF
}

// handleEnter 함수가 더 이상 EventLoop에서 필요하지 않으므로,
// Client 구조체에서 관련 메서드인 handleEnter는 삭제하는 것이 깔끔하지만,
// 현재 코드를 유지하기 위해 body를 비워둡니다.
