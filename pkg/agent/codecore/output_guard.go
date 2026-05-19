package codecore

import (
	"os"
	"sync"
)

type OutputGuard struct {
	mu          sync.Mutex
	originalOut *os.File
	originalErr *os.File
	reader      *os.File
	writer      *os.File
	active      bool
	captured    []byte
}

func NewOutputGuard() *OutputGuard { return &OutputGuard{} }

func (g *OutputGuard) TakeOverStdout() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active {
		return nil
	}
	g.originalOut = os.Stdout
	g.originalErr = os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	g.reader = reader
	g.writer = writer
	os.Stdout = writer
	os.Stderr = writer
	g.active = true
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				return
			}
			g.mu.Lock()
			g.captured = append(g.captured, buf[:n]...)
			g.mu.Unlock()
		}
	}()
	return nil
}

func (g *OutputGuard) RestoreStdout() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.active {
		return
	}
	if g.originalOut != nil {
		os.Stdout = g.originalOut
	}
	if g.originalErr != nil {
		os.Stderr = g.originalErr
	}
	if g.writer != nil {
		g.writer.Close()
	}
	if g.reader != nil {
		g.reader.Close()
	}
	g.active = false
}

func (g *OutputGuard) GetCapturedOutput() []byte {
	g.mu.Lock()
	defer g.mu.Unlock()
	r := make([]byte, len(g.captured))
	copy(r, g.captured)
	return r
}

func (g *OutputGuard) IsActive() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active
}
