package ui

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

func TestEscreverOSC52(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("worker"))
	casos := []struct {
		nome   string
		vars   map[string]string
		prefix string
	}{
		{"terminal", nil, "\x1b]52;c;" + b64},
		{"tmux", map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0"}, "\x1bPtmux;\x1b\x1b]52;c;" + b64},
		{"screen", map[string]string{"TERM": "screen-256color"}, "\x1bP\x1b]52;c;" + b64},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			var buf bytes.Buffer
			if err := EscreverOSC52(&buf, func(k string) string { return c.vars[k] }, "worker"); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(buf.String(), c.prefix) {
				t.Errorf("sequência %q, esperado o começo %q", buf.String(), c.prefix)
			}
		})
	}
}
