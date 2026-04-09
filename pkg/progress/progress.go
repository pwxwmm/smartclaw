package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Spinner struct {
	frames   []string
	current  int
	message  string
	active   bool
	mu       sync.Mutex
	stopChan chan struct{}
	writer   io.Writer
}

var defaultFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames:   defaultFrames,
		message:  message,
		stopChan: make(chan struct{}),
		writer:   os.Stderr,
	}
}

func (s *Spinner) Start() {
	s.mu.Lock()
	s.active = true
	s.mu.Unlock()

	go func() {
		for {
			select {
			case <-s.stopChan:
				return
			default:
				s.render()
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.stopChan)

	s.clear()
}

func (s *Spinner) Update(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.message = message
}

func (s *Spinner) render() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	frame := s.frames[s.current]
	s.current = (s.current + 1) % len(s.frames)

	s.clear()
	fmt.Fprintf(s.writer, "%s %s", frame, s.message)
}

func (s *Spinner) clear() {
	fmt.Fprintf(s.writer, "\r%s\r", "                    ")
}

type ProgressBar struct {
	total   int64
	current int64
	width   int
	message string
	mu      sync.Mutex
	writer  io.Writer
}

func NewProgressBar(total int64, message string) *ProgressBar {
	return &ProgressBar{
		total:   total,
		current: 0,
		width:   40,
		message: message,
		writer:  os.Stderr,
	}
}

func (p *ProgressBar) Add(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current += n
	if p.current > p.total {
		p.current = p.total
	}

	p.render()
}

func (p *ProgressBar) Set(n int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = n
	if p.current > p.total {
		p.current = p.total
	}

	p.render()
}

func (p *ProgressBar) Done() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.current = p.total
	p.render()
	fmt.Fprintln(p.writer)
}

func (p *ProgressBar) render() {
	percent := float64(p.current) / float64(p.total)
	filled := int(float64(p.width) * percent)

	bar := ""
	for i := 0; i < p.width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	fmt.Fprintf(p.writer, "\r%s [%s] %.0f%% (%d/%d)", p.message, bar, percent*100, p.current, p.total)
}

type Progress interface {
	Start()
	Stop()
}

type NoOpProgress struct{}

func (n *NoOpProgress) Start() {}
func (n *NoOpProgress) Stop()  {}

func WithSpinner(message string, fn func() error) error {
	spinner := NewSpinner(message)
	spinner.Start()
	defer spinner.Stop()

	return fn()
}

func WithProgress(total int64, message string, fn func(*ProgressBar) error) error {
	bar := NewProgressBar(total, message)
	defer fmt.Fprintln(os.Stderr)

	return fn(bar)
}

type Counter struct {
	count int
	mu    sync.Mutex
}

func NewCounter() *Counter {
	return &Counter{}
}

func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count++
}

func (c *Counter) Add(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count += n
}

func (c *Counter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func (c *Counter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.count = 0
}
