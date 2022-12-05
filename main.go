package main

import (
	"fmt"
	"go-deb/internal/deb"
)

const (
	MIRROR = "https://mirrors.kernel.org/debian"
)

var (
	DISTS = []string{
		"stretch",
		"stretch-backports",
		"stretch-updates",
		"sid",
		"sid-backports",
		"sid-updates",
		"jessie",
		"jessie-backports",
		"jessie-udpates",
		"buster",
		"buster-backports",
		"buster-updates",
		"bullseye",
		"bullseye-backports",
		"bullseye-updates",
	}
)

func main() {
	packages, err := deb.GetPackages("linux-headers", "kernel", MIRROR, DISTS)
	if err != nil {
		panic(err)
	}

	releases, err := deb.GetReleasesFromPackages(packages)
	if err != nil {
		panic(err)
	}

	if len(releases) > 0 {
		for _, v := range releases {
			fmt.Println(v)
		}
	}
}
