package util

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/laindream/go-callflow-vis/ir"
	"os"
	"path/filepath"
	"strings"
)

func GetFuncSimpleName(name, prefix string) string {
	//if strings.Contains(name, ".") {
	//	return name[strings.LastIndex(name, ".")+1:]
	//}
	return strings.ReplaceAll(name, prefix, "")
}

func GetSiteSimpleName(name, prefix string) string {
	//if strings.Contains(name, "(") {
	//	return name[:strings.Index(name, "(")]
	//}
	return strings.ReplaceAll(name, prefix, "")
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

func GetSimpleEdgeForPath(path []*ir.Edge) *ir.Edge {
	if len(path) == 0 {
		return nil
	}
	if len(path) == 1 {
		return path[0]
	}
	caller := path[0].GetCaller()
	callee := path[len(path)-1].GetCallee()
	site := fmt.Sprintf("%s->...->%s", path[0].GetSite().GetName(), path[len(path)-1].GetSite().GetName())
	return &ir.Edge{Caller: caller, Callee: callee, Site: &ir.Site{Name: site}}
}
