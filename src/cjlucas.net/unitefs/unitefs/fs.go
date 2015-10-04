package unitefs

import (
	"fmt"
	"os"
	"path"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

type Node struct {
	Name     string
	RealPath string
	Inode    uint64
	Size     uint64
	Mtime    time.Time
	Ctime    time.Time
	Mode     os.FileMode
	Tree     *Tree
}

func timeFromTimespec(ts syscall.Timespec) time.Time {
	sec, nsec := ts.Unix()
	return time.Unix(sec, nsec)
}

func newNodeForPath(fpath string) (*Node, error) {
	stat := syscall.Stat_t{}
	if err := syscall.Stat(fpath, &stat); err != nil {
		return nil, err
	}

	node := Node{}
	node.RealPath = path.Clean(fpath)
	_, node.Name = path.Split(node.RealPath)
	node.Size = uint64(stat.Size)
	node.Mtime = timeFromTimespec(stat.Mtimespec)
	node.Ctime = timeFromTimespec(stat.Ctimespec)
	node.Mode = os.FileMode(stat.Mode)

	return &node, nil
}

func (n Node) Children() []Node {
	return n.Tree.Nodes[n]
}

func (n Node) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = n.Inode
	a.Mode = n.Mode
	a.Size = n.Size
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

func (n Node) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	fmt.Println("Flush")
	/*
	 *if fp, ok := masterHandleMap[req.Handle]; ok {
	 *    delete(masterHandleMap, req.Handle)
	 *    return fp.Close()
	 *}
	 */

	return nil
}

func (n Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	/*
	 *if fp, ok := masterHandleMap[req.Handle]; fp == nil || !ok {
	 *    masterHandleMap.Open(req.Handle, n.RealPath)
	 *}
	 */

	resp.Data = make([]byte, req.Size)
	//masterHandleMap.ReadAt(req.Handle, req.Offset, resp.Data)
	return nil
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

type NodeChildMap map[Node][]Node

type Tree struct {
	Root  Node
	Nodes NodeChildMap
}

func (t Tree) Add(node Node) {
	t.Nodes[node] = make([]Node, 0)
}

type UnionTree struct {
	Trees      map[string]Tree
	usedInodes []uint64
}

type FS struct {
	UnionTree Tree
	HandleMap map[fuse.HandleID]*os.File
}

func (fs FS) Root() (fs.Node, error) {
	return fs.UnionTree.Root, nil
}
