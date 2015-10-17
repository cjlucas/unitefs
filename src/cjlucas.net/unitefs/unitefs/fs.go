package unitefs

import (
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"syscall"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

type handleManager interface {
	GetHandle(id fuse.HandleID) *os.File
	RegisterHandle(id fuse.HandleID, fp *os.File)
	UnregisterHandle(id fuse.HandleID)
}

type inodeManager interface {
	NextInode() uint64
}

type Node struct {
	Name          string
	RealPath      string
	Inode         uint64
	Size          uint64
	Mtime         time.Time
	Ctime         time.Time
	Mode          os.FileMode
	Tree          *Tree
	handleManager handleManager
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

func (n Node) asDirent() fuse.Dirent {
	var d fuse.Dirent

	d.Name = n.Name
	d.Inode = n.Inode
	if os.ModeDir&n.Mode > 0 {
		d.Type = fuse.DT_Dir
	} else {
		d.Type = fuse.DT_File
	}

	return d
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
	n.handleManager.UnregisterHandle(req.Handle)

	return nil
}

func (n Node) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if fp := n.handleManager.GetHandle(req.Handle); fp != nil {
		resp.Data = make([]byte, req.Size)
		_, err := fp.ReadAt(resp.Data, req.Offset)
		return err
	}

	return syscall.EBADF
}

func (n Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	fmt.Println("ReadDirAll", n)
	children := n.Children()
	dirs := make([]fuse.Dirent, len(children))

	for i := range dirs {
		dirs[i] = children[i].asDirent()
	}

	fmt.Println(dirs)
	return dirs, nil
}

type NodeChildMap map[Node][]Node

type Tree struct {
	Root          Node
	Nodes         NodeChildMap
	handleManager handleManager
	inodeManager  inodeManager
}

func newTree(name string) *Tree {
	t := Tree{}
	t.Nodes = make(NodeChildMap)
	return &t
}

func (t Tree) Add(node Node) {
	t.Nodes[node] = make([]Node, 0)
}

func (t *Tree) walker(fpath string, info os.FileInfo, err error) error {
	n, _ := newNodeForPath(fpath)
	n.handleManager = t.handleManager
	n.Tree = t
	n.Inode = t.inodeManager.NextInode()

	fmt.Printf("%#v\n", n)

	return nil
}

func (t *Tree) getLevel(level int) []Node {
	parents := []Node{t.Root}

	for i := 0; i <= level; i++ {
		children := make([]Node, 0)
		for _, n := range parents {
			for _, c := range n.Children() {
				children = append(children, c)
			}
		}

		parents = children
	}

	return parents
}

type UnionTree struct {
	Tree

	Subtrees map[string]*Tree
}

func (t *UnionTree) Build() error {
	return nil
}

type FS struct {
	UnionTree  UnionTree
	handleMap  map[fuse.HandleID]*os.File
	usedInodes []uint64
}

// Implement inodeManager
func (fs *FS) NextInode() uint64 {
	isUsed := func(n uint64) bool {
		for _, inode := range fs.usedInodes {
			if n == inode {
				return true
			}
		}

		return false
	}

	for {
		n := uint64(rand.Int63())
		if !isUsed(n) {
			return n
		}
	}

}

func NewFS() *FS {
	ufs := FS{}
	ufs.UnionTree.Subtrees = make(map[string]*Tree)
	return &ufs
}

func (fs *FS) GetHandle(id fuse.HandleID) *os.File {
	if fp := fs.handleMap[id]; fp != nil {
		return fp
	} else {
		return nil
	}
}

// Implement HandleManager
func (fs *FS) RegisterHandle(id fuse.HandleID, fp *os.File) {
	if _, ok := fs.handleMap[id]; ok {
		panic(fmt.Sprintf("Handle with id %d already registered", id))
	}

	fs.handleMap[id] = fp
}

func (fs *FS) UnregisterHandle(id fuse.HandleID) {
	if _, ok := fs.handleMap[id]; !ok {
		panic(fmt.Sprintf("Handle with %d not registered", id))
	}

	delete(fs.handleMap, id)
}

func (fs *FS) RegisterSubtree(treeRoot string) {
	treeRoot = path.Clean(treeRoot)
	t := newTree(treeRoot)
	t.inodeManager = fs
	filepath.Walk(treeRoot, t.walker)

	fs.UnionTree.Subtrees[treeRoot] = t
}

func (fs FS) Root() (fs.Node, error) {
	return fs.UnionTree.Root, nil
}
