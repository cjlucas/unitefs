package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type Node struct {
	Name  string
	Inode uint64
	Mode  os.FileMode
}

func (n Node) Children() []Node {
	return tree.Nodes[n]
}

func (n Node) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = n.Inode
	a.Mode = n.Mode
	a.Size = 0
	return nil
}

func (n Node) Lookup(ctx context.Context, name string) (fs.Node, error) {
	for _, c := range n.Children() {
		if name == c.Name {
			return c, nil
		}
	}

	return nil, fuse.ENOENT
}

func (n Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	fmt.Println("ReadDirAll", n)
	children := n.Children()
	dirs := make([]fuse.Dirent, len(children))

	for i := range dirs {
		c := children[i]
		dirs[i].Name = c.Name
		dirs[i].Inode = c.Inode
		if os.ModeDir&c.Mode > 0 {
			dirs[i].Type = fuse.DT_Dir
		} else {
			dirs[i].Type = fuse.DT_File
		}
	}

	fmt.Println(dirs)
	return dirs, nil
}

func NewNode(name string, inode uint64) Node {
	return Node{Name: name, Inode: inode}
}

type NodeChildMap map[Node][]Node
type Tree struct {
	Root  Node
	Nodes NodeChildMap
}

func (t Tree) Add(node Node) {
	t.Nodes[node] = make([]Node, 0)
}

var tree Tree
var nextInode = uint64(2)

func RootWalker(pathStr string, info os.FileInfo, err error) error {
	pathStr = path.Clean(pathStr)
	rootPath := path.Clean(os.Args[1])

	if len(pathStr) == len(rootPath) {
		tree.Root = NewNode("/", 1)
		tree.Root.Mode = os.ModeDir | 0777
		tree.Nodes = make(NodeChildMap)
		tree.Add(tree.Root)
		return nil
	}

	pathRel := pathStr[len(rootPath)+1:]
	pathSplit := strings.Split(pathRel, "/")

	fmt.Println(pathSplit)

	parent := tree.Root
	for i := range pathSplit {
		name := pathSplit[i]

		if i == len(pathSplit)-1 {
			node := NewNode(name, nextInode)
			node.Name = name
			node.Mode = info.Mode()

			tree.Nodes[parent] = append(tree.Nodes[parent], node)
			tree.Add(node)
			nextInode++
			break
		}

		for _, c := range tree.Nodes[parent] {
			if name == c.Name {
				parent = c
			}
		}
	}

	return nil
}

func main() {
	src := path.Clean(os.Args[1])

	filepath.Walk(src, RootWalker)
	fmt.Println(tree)

	c, err := fuse.Mount(
		os.Args[2],
		fuse.FSName("unitefs"),
		fuse.Subtype("unitefs"),
		fuse.LocalVolume(),
		fuse.VolumeName("unitefs"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	err = fs.Serve(c, FS{})
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

type FS struct{}

func (fs FS) Root() (fs.Node, error) {
	return tree.Root, nil
}
