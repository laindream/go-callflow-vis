package util

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func GetFuncSimpleName(name string) string {
	if strings.Contains(name, ".") {
		return name[strings.LastIndex(name, ".")+1:]
	}
	return name

}

func Escape(escape string) string {
	escape = strings.ReplaceAll(escape, "\\", "\\\\")
	escape = strings.ReplaceAll(escape, "\"", "\\\"")
	return escape
}

func WriteToFile(content, filename string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	if err != nil {
		return err
	}
	return nil
}

func GetHash(o interface{}) string {
	jsonStr, _ := json.Marshal(o)
	bytes := md5.Sum(jsonStr)
	return hex.EncodeToString(bytes[:])
}
