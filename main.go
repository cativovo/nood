package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/labstack/echo"
)

type Node struct {
	Children map[string]*Node
	Name     string
	Path     string
	IsDir    bool
}

func (n *Node) Add(path string, d fs.DirEntry) {
	parts := strings.Split(path, "/")
	node := Node{
		Children: make(map[string]*Node),
		Name:     d.Name(),
		Path:     path,
		IsDir:    d.IsDir(),
	}

	if len(parts) == 1 {
		n.Children[path] = &node
		return
	}

	p := n.Children[parts[0]]

	for _, part := range parts[1:] {
		if p.Children[part] == nil {
			p.Children[part] = &node
			break
		}

		p = p.Children[part]
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

	err := fs.WalkDir(fileSystem, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Fatal(err)
		}

		if path == "." {
			return nil
		}

		tree.Node.Add(path, d)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	Print(tree.Node, "")

	e := echo.New()
	e.Static("/media", "media")
	e.GET("/", func(ctx echo.Context) error {
		return ctx.Redirect(http.StatusMovedPermanently, "/home/")
	})
	e.GET("/home/*", homeHandler)
	e.Logger.Fatal(e.Start("127.0.0.1:9000"))
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

	return c.JSON(http.StatusOK, node)
}

}
