package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cjlucas.net/unitefs/unitefs"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type HandleMap map[fuse.HandleID]*os.File

var masterHandleMap HandleMap

func (m HandleMap) Open(id fuse.HandleID, name string) (*os.File, error) {
	if _, ok := m[id]; ok {
		panic(fmt.Sprintf("Handle with id %d already present in map\n", id))
	}

	fp, err := os.Open(name)

	if err == nil {
		m[id] = fp
	}

	return fp, err
}

func (m HandleMap) Close(id fuse.HandleID) error {
	return m[id].Close()
}

func (m HandleMap) ReadAt(id fuse.HandleID, offset int64, buf []byte) (int, error) {
	if fp, ok := m[id]; ok {
		return fp.ReadAt(buf, offset)
	}

	panic(fmt.Sprintf("Handle with id %d not found in map", id))
}

var tree fs.Tree
var nextInode = uint64(2)

func RootWalker(pathStr string, info os.FileInfo, err error) error {
	pathStr = path.Clean(pathStr)
	rootPath := path.Clean(os.Args[1])

	if len(pathStr) == len(rootPath) {
		tree.Root = unitefs.NewNode("/", 1)
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
			node := unitefs.NewNode(name, nextInode)
			node.Name = name
			node.Mode = info.Mode()
			node.Size = uint64(info.Size())
			node.RealPath = pathStr

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
	masterHandleMap = make(HandleMap)

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

	err = fs.Serve(c, unitefs.FS{})
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}
