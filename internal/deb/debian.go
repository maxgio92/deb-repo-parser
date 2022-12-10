package deb

import (
	log "github.com/sirupsen/logrus"
	"sync"

	progressbar "github.com/schollz/progressbar/v3"
	"pault.ag/go/archive"
)

const (
	INRELEASE             = "InRelease"
	PACKAGES_INDEX_FORMAT = ".xz"

	EXCLUDE_INSTALLERS              = true
	FAIL_FOR_MISSING_PACKAGES_INDEX = true
	FAIL_FOR_MISSING_RELEASE_INDEX  = false
)

func GetPackages(packageName string, packageSection string, mirrorURL string, dists []string) ([]archive.Package, error) {
	packages := []archive.Package{}

	perDistWG := sync.WaitGroup{}
	perDistWG.Add(len(dists))

	packagesCh := make(chan []archive.Package)

	errCh := make(chan error)

	done := make(chan bool, 1)

	bar := progressbar.Default(int64(len(dists)), "Total")

	// Run producer workers.
	for _, v := range dists {
		dist := v
		go getDistPackages(bar, &perDistWG, packagesCh, errCh, packageName, packageSection, mirrorURL, dist)
	}

	// Run consumer worker.
	go consumePackages(done, &packages, packagesCh, errCh)

	// Wait for producers to complete.
	perDistWG.Wait()
	close(packagesCh)
	close(errCh)

	// Wait for consumers to complete.
	<-done

	return packages, nil
}

func consumePackages(done chan bool, packages *[]archive.Package, packagesCh chan []archive.Package, errCh chan error) {
	for errCh != nil || packagesCh != nil {
		select {
		case p, ok := <-packagesCh:
			if ok {
				log.Info("Scanned DB")
				if len(p) > 0 {
					*packages = append(*packages, p...)
					log.Infof("New %d packages found", len(p))
				}
				continue
			}
			packagesCh = nil
		case e, ok := <-errCh:
			if ok {
				log.Error(e)
				continue
			}
			errCh = nil
		}
	}
	done <- true
}
