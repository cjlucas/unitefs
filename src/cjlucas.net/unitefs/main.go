package main

import (
	"fmt"
	"log"
	"os"

	"cjlucas.net/unitefs/unitefs"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

func main() {
	ufs := unitefs.NewFS()

	for i := 1; i < len(os.Args)-1; i++ {
		println(os.Args[i])
		ufs.RegisterSubtree(os.Args[i])
	}

	c, err := fuse.Mount(
		os.Args[len(os.Args)-1],
		fuse.FSName("unitefs"),
		fuse.Subtype("unitefs"),
		fuse.LocalVolume(),
		fuse.VolumeName("unitefs"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	fmt.Println("FS mounted")

	err = fs.Serve(c, ufs)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}
