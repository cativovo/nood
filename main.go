package main

import (
	"cmp"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Node struct {
	Children map[string]*Node
	Name     string
	Path     string
	MimeType string
	IsDir    bool
}

func (n *Node) Add(p string, d fs.DirEntry) {
	parts := strings.Split(p, "/")
	var mimeType string

	if !d.IsDir() {
		mimeType = mime.TypeByExtension(path.Ext(p))
	}

	node := Node{
		Children: make(map[string]*Node),
		Name:     d.Name(),
		Path:     p,
		IsDir:    d.IsDir(),
		MimeType: mimeType,
	}

	if len(parts) == 1 {
		n.Children[p] = &node
		return
	}

	c := n.Children[parts[0]]

	for _, part := range parts[1:] {
		if c.Children[part] == nil {
			c.Children[part] = &node
			break
		}

		c = c.Children[part]
	}
}

type Tree struct {
	Node Node
	mu   sync.Mutex
}

func Print(n Node, indent string) {
	fmt.Println(indent, n.Name)
	for _, v := range n.Children {
		Print(*v, " "+indent)
	}
}

var tree Tree

func main() {
	root := "./media/"
	fileSystem := os.DirFS(root)
	baseNode := Node{
		Children: make(map[string]*Node),
		Name:     "root",
		Path:     ".",
		IsDir:    true,
	}
	tree.Node = baseNode

	err := fs.WalkDir(fileSystem, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatal(err)
		}

		if p == "." {
			return nil
		}

		tree.Node.Add(p, d)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	Print(tree.Node, "")

	e := echo.New()
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: `{"time":"${time_rfc3339_nano}","id":"${id}","remote_ip":"${remote_ip}",` +
			`"host":"${host}","method":"${method}","uri":"${uri}","user_agent":"${user_agent}",` +
			`"status":${status},"error":"${error}","latency":${latency},"latency_human":"${latency_human}"` +
			`,"bytes_in":${bytes_in},"bytes_out":${bytes_out}}` + "\n",
		CustomTimeFormat: "2006-01-02 15:04:05.00000",
	}))
	e.Static("/media", "media")
	e.GET("/", func(ctx echo.Context) error {
		return ctx.Redirect(http.StatusMovedPermanently, "/home/")
	})
	e.GET("/home/*", homeHandler)
	e.Logger.Fatal(e.Start(":9000"))
}

func homeHandler(c echo.Context) error {
	tree.mu.Lock()
	defer tree.mu.Unlock()

	var node *Node

	paramValues := strings.Split(c.ParamValues()[0], "/")

	if paramValues[0] != "" {
		node = tree.Node.Children[paramValues[0]]

		if len(paramValues) > 1 {
			for _, v := range paramValues[1:] {
				node = node.Children[v]
				if !node.IsDir {
					break
				}
			}
		}
	} else {
		node = &tree.Node
	}

	nodes := make([]Node, 0, len(node.Children))

	for _, v := range node.Children {
		nodes = append(nodes, *v)
	}

	slices.SortFunc(nodes, func(a, b Node) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return Render(c, http.StatusOK, Home(nodes, strings.TrimSuffix(c.Request().URL.Path, "/")))
}

func Render(c echo.Context, statusCode int, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(c.Request().Context(), buf); err != nil {
		return err
	}

	return c.HTML(statusCode, buf.String())
}
