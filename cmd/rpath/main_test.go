package main_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEndToEnd(t *testing.T) {
	e := newExecutor(t)
	defer e.close()

	if err := run(e.cmd, "-h"); err != nil {
		t.Fatalf("%s help %v", e.cmd, err)
	}

	t.Run("yaml", func(t *testing.T) {
		const document = `apiVersion: v1
kind: Text
metadata:
  name: sometext
spec:
  text1: テキスト
  text2: text`
		docFile := fmt.Sprintf("%s/document.yml", t.TempDir())
		if err := func() error {
			f, err := os.Create(docFile)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.WriteString(f, document)
			return err
		}(); err != nil {
			t.Fatal(err)
			return
		}

		for _, tc := range []struct {
			title  string
			line   int
			column int
			offset int
			want   string
		}{
			{
				title:  "offset",
				offset: 33,
				want:   `$.metadata`,
			},
			{
				title:  "second line",
				line:   2,
				column: 7,
				offset: -1,
				want:   `$.kind`,
			},
			{
				title:  "first char",
				line:   1,
				column: 1,
				offset: -1,
				want:   `$.apiVersion`,
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				out, err := exec.Command(
					e.cmd,
					"-line", fmt.Sprint(tc.line),
					"-column", fmt.Sprint(tc.column),
					"-offset", fmt.Sprint(tc.offset),
					"yaml",
					docFile,
				).Output()
				assert.Nil(t, err)
				assert.Equal(t, tc.want, string(out))
			})
		}
	})
}

func run(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type executor struct {
	dir string
	cmd string
}

func newExecutor(t *testing.T) *executor {
	t.Helper()
	e := &executor{}
	e.init(t)
	return e
}

func (e *executor) init(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "rpath")
	if err != nil {
		t.Fatal(err)
	}
	cmd := filepath.Join(dir, "rpath")
	// build rpath command
	if err := run("go", "build", "-o", cmd); err != nil {
		t.Fatal(err)
	}
	e.dir = dir
	e.cmd = cmd
}

func (e *executor) close() {
	os.RemoveAll(e.dir)
}
