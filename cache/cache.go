package cache

import (
	"bufio"
	"fmt"
	"github.com/apache/incubator-fury/go/fury"
	"go-callflow-vis/ir"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	defaultCachePath = "./.go_callflow_vis_cache"
	furyC            *fury.Fury
)

func GetFury() *fury.Fury {
	return furyC
}

func init() {
	if furyC != nil {
		return
	}
	fr := fury.NewFury(true)
	fr.RegisterTagType("ir.Callgraph", ir.Callgraph{})
	fr.RegisterTagType("ir.Node", ir.Node{})
	fr.RegisterTagType("ir.Edge", ir.Edge{})
	fr.RegisterTagType("ir.Func", ir.Func{})
	fr.RegisterTagType("ir.Site", ir.Site{})
	furyC = fr
}

type FileCache struct {
	path string
}

func NewFileCache(path string) *FileCache {
	if path == "" {
		path = defaultCachePath
	}
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	return &FileCache{path: path}
}

func (f *FileCache) Set(key string, value interface{}) error {
	filename := fmt.Sprintf("%s/%s.fury", f.path, key)
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	bytes, err := GetFury().Marshal(value)
	if err != nil {
		return err
	}
	_, err = writer.Write(bytes)
	if err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return nil
}

func (f *FileCache) Get(key string, valuePtr interface{}) error {
	file, err := os.Open(fmt.Sprintf("%s/%s.fury", f.path, key))
	if err != nil {
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	bytes, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("Error reading from reader: %v", err)
		return err
	}
	err = GetFury().Unmarshal(bytes, valuePtr)
	if err != nil {
		return err
	}
	return nil
}
