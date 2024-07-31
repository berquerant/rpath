package main_test

import (
	"fmt"
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

	t.Run("json", func(t *testing.T) {
		const docFile = "./test/test.json"
		for _, tc := range []struct {
			title  string
			line   int
			column int
			offset int
			want   string
		}{
			{
				title:  "last key",
				line:   19,
				column: 5,
				offset: -1,
				want:   `.["spec"]["text3"]`,
			},
			{
				title:  "nested object array",
				line:   15,
				column: 11,
				offset: -1,
				want:   `.["spec"]["texts"][2]["t4"][0]`,
			},
			{
				title:  "array head tail",
				line:   11,
				column: 11,
				offset: -1,
				want:   `.["spec"]["texts"][0]`,
			},
			{
				title:  "array head head",
				line:   11,
				column: 7,
				offset: -1,
				want:   `.["spec"]["texts"][0]`,
			},
			{
				title:  "nested value",
				line:   5,
				column: 17,
				offset: -1,
				want:   `.["metadata"]["name"]`,
			},
			{
				title:  "offset",
				offset: 47,
				want:   `.["metadata"]`,
			},
			{
				title:  "value",
				line:   3,
				column: 15,
				offset: -1,
				want:   `.["kind"]`,
			},
			{
				title:  "key",
				line:   3,
				column: 3,
				offset: -1,
				want:   `.["kind"]`,
			},
			{
				title:  "last char",
				line:   21,
				column: 1,
				offset: -1,
				// FIXME: for now, display the last element encountered
				want: `.["spec"]["text3"]`,
			},
			{
				title:  "first char",
				line:   1,
				column: 1,
				offset: -1,
				want:   `.`,
			},
		} {
			t.Run(tc.title, func(t *testing.T) {
				cmd := exec.Command(
					e.cmd,
					"-line", fmt.Sprint(tc.line),
					"-column", fmt.Sprint(tc.column),
					"-offset", fmt.Sprint(tc.offset),
					// "-debug",
					"json",
					docFile,
				)
				cmd.Stderr = os.Stderr
				out, err := cmd.Output()
				assert.Nil(t, err)
				assert.Equal(t, tc.want, string(out), "%d:%d[%d]", tc.line, tc.column, tc.offset)
			})
		}
	})

	t.Run("yaml", func(t *testing.T) {
		const docFile = "./test/test.yaml"

		for _, tc := range []struct {
			title  string
			line   int
			column int
			offset int
			want   string
		}{
			{
				title:  "address of second document",
				line:   29,
				column: 15,
				offset: -1,
				want:   `$.spec.address`,
			},
			{
				title:  "kind of second document",
				line:   24,
				column: 8,
				offset: -1,
				want:   `$.kind`,
			},
			{
				title:  "first char of second document",
				line:   23,
				column: 1,
				offset: -1,
				want:   `$.apiVersion`,
			},
			{
				title:  "last element value",
				line:   21,
				column: 12,
				offset: -1,
				want:   `$.spec.text4`,
			},
			{
				title:  "array element document2",
				line:   20,
				column: 7,
				offset: -1,
				want:   `$.spec.texts3[0]`,
			},
			{
				title:  "array element document line2",
				line:   16,
				column: 6,
				offset: -1,
				want:   `$.spec.texts2[1]`,
			},
			{
				title:  "array element document line1",
				line:   15,
				column: 6,
				offset: -1,
				want:   `$.spec.texts2[1]`,
			},
			{
				title:  "after array",
				line:   11,
				column: 4,
				offset: -1,
				want:   `$.spec.text3`,
			},
			{
				title:  "array element0 tail",
				line:   9,
				column: 8,
				offset: -1,
				want:   `$.spec.texts[0]`,
			},
			{
				title:  "array element0 head",
				line:   9,
				column: 7,
				offset: -1,
				want:   `$.spec.texts[0]`,
			},
			{
				title:  "array element1",
				line:   10,
				column: 7,
				offset: -1,
				want:   `$.spec.texts[1]`,
			},
			{
				title:  "value tail",
				line:   7,
				column: 12,
				offset: -1,
				want:   `$.spec.text2`,
			},
			{
				title:  "value head",
				line:   7,
				column: 10,
				offset: -1,
				want:   `$.spec.text2`,
			},
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
				cmd := exec.Command(
					e.cmd,
					"-line", fmt.Sprint(tc.line),
					"-column", fmt.Sprint(tc.column),
					"-offset", fmt.Sprint(tc.offset),
					// "-debug",
					"yaml",
					docFile,
				)
				cmd.Stderr = os.Stderr
				out, err := cmd.Output()
				assert.Nil(t, err)
				assert.Equal(t, tc.want, string(out),
					"%d:%d[%d]", tc.line, tc.column, tc.offset)
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
