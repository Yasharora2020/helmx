package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateResultMsg(t *testing.T) {
	tests := []struct {
		name       string
		msg        templateResultMsg
		isError    bool
		hasPath    bool
		pathPrefix string
	}{
		{
			name:       "success result",
			msg:        templateResultMsg{outputPath: "./output.yaml", err: nil},
			isError:    false,
			hasPath:    true,
			pathPrefix: ".",
		},
		{
			name:       "error result",
			msg:        templateResultMsg{outputPath: "", err: os.ErrNotExist},
			isError:    true,
			hasPath:    false,
			pathPrefix: "",
		},
		{
			name:       "success with subdirectory",
			msg:        templateResultMsg{outputPath: "./rendered/nginx.yaml", err: nil},
			isError:    false,
			hasPath:    true,
			pathPrefix: "rendered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if (tt.msg.err != nil) != tt.isError {
				t.Errorf("isError: got %v, want %v", tt.msg.err != nil, tt.isError)
			}
			if (tt.msg.outputPath != "") != tt.hasPath {
				t.Errorf("hasPath: got %v, want %v", tt.msg.outputPath != "", tt.hasPath)
			}
			if tt.hasPath && tt.pathPrefix != "" {
				dir := filepath.Dir(tt.msg.outputPath)
				if dir != tt.pathPrefix {
					t.Errorf("pathPrefix: got %s, want %s", dir, tt.pathPrefix)
				}
			}
		})
	}
}
