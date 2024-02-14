package go_imports

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goghcrow/go-loader"
	"golang.org/x/tools/txtar"
)

func TestOptimizeImports(t *testing.T) {
	files, err := filepath.Glob("testdata/import/*.txt")
	fatalIf(t, err)

	for _, testFile := range files {
		t.Log(testFile)
		ar, err := txtar.ParseFile(testFile)
		fatalIf(t, err)

		dir := t.TempDir()
		err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module a.b.c\n"), 0666)
		fatalIf(t, err)

		var wants = map[string]string{}
		for _, f := range ar.Files {
			if filepath.Ext(f.Name) == ".optimized" {
				wants[f.Name] = string(f.Data)
				continue
			}

			filename := filepath.Join(dir, f.Name)
			err = os.MkdirAll(filepath.Dir(filename), 0777)
			fatalIf(t, err)
			err = os.WriteFile(filename, f.Data, 0666)
			fatalIf(t, err)
		}

		l := loader.MustNew(dir, loader.WithLoadDepts()) // mustLoadDepts

		l.VisitAllFiles(func(f *File) {
			name := f.Filename[len(dir)+1:]

			want, ok := wants[name+".optimized"]
			if !ok {
				return
			}

			t.Run(testFile+"/"+name, func(t *testing.T) {
				Optimize(l, f, "a.b.c/project", []string{"a.b.c/company"})
				have := l.FormatFile(f.File)

				if strings.TrimSpace(want) != strings.TrimSpace(have) {
					diff := Diff("have.go", []byte(have), "want.go", []byte(want))
					fmt.Println(string(diff))
					t.Errorf("stdout:\n")
					println(have)
					t.Errorf("want:\n")
					println(want)
				}
			})
		})
	}
}

func Diff(oldName string, old []byte, newName string, new []byte) []byte {
	writeTempFile := func(data []byte) (string, error) {
		file, err := os.CreateTemp("", "diff")
		if err != nil {
			return "", err
		}
		_, err = file.Write(data)
		if err1 := file.Close(); err == nil {
			err = err1
		}
		if err != nil {
			_ = os.Remove(file.Name())
			return "", err
		}
		return file.Name(), nil
	}

	f1, err := writeTempFile(old)
	panicIf(err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f1)

	f2, err := writeTempFile(new)
	panicIf(err)
	//goland:noinspection GoUnhandledErrorResult
	defer os.Remove(f2)

	data, err := exec.Command("diff", "-u", f1, f2).CombinedOutput()
	if err != nil && len(data) == 0 {
		panicIf(err)
	}

	if len(data) == 0 {
		return nil
	}

	i := bytes.IndexByte(data, '\n')
	if i < 0 {
		return data
	}
	j := bytes.IndexByte(data[i+1:], '\n')
	if j < 0 {
		return data
	}
	start := i + 1 + j + 1
	if start >= len(data) || data[start] != '@' {
		return data
	}

	return append([]byte(fmt.Sprintf("diff %s %s\n--- %s\n+++ %s\n", oldName, newName, oldName, newName)), data[start:]...)
}

func fatalIf(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
