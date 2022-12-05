package deb

import (
	"fmt"
	progressbar "github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"pault.ag/go/archive"
	"pault.ag/go/debian/deb"
	"strings"
	"sync"
)

func init() {
	log.SetLevel(log.FatalLevel)
	log.SetFormatter(&log.TextFormatter{
		ForceColors:      true,
		DisableTimestamp: true,
	})
}

func getDistPackages(bar *progressbar.ProgressBar, waitGroup *sync.WaitGroup, packagesCh chan []archive.Package, errCh chan error, packageName string, packageSection string, mirrorURL string, dist string) {
	defer waitGroup.Done()
	defer bar.Add(1)

	indexesWG := sync.WaitGroup{}

	packagesInternalCh := make(chan []archive.Package)

	errInternalCh := make(chan error)

	done := make(chan bool, 1)

	distURL, err := url.JoinPath(mirrorURL, "dists", dist)
	if err != nil {
		errCh <- err
		return
	}

	inRelease, err := getInReleaseFromDistURL(distURL)
	if err != nil {
		errCh <- err
		return
	}

	indexPaths := []string{}
	for _, v := range inRelease.MD5Sum {
		if strings.Contains(v.Filename, "Packages"+PACKAGES_INDEX_FORMAT) {
			indexPaths = append(indexPaths, v.Filename)
		}
	}

	indexesWG.Add(len(indexPaths))
	internalBar := progressbar.Default(int64(len(indexPaths)), fmt.Sprintf("Getting index for dist %s", dist))

	// Run producer workers.
	for _, v := range indexPaths {
		if EXCLUDE_INSTALLERS && strings.Contains(v, "debian-installer") {
			continue
		}
		if strings.Contains(v, "non-free") {
			continue
		}

		indexURL, err := url.JoinPath(mirrorURL, "dists", dist, v)
		if err != nil {
			errCh <- err
			return
		}

		go getIndexPackages(internalBar, &indexesWG, packagesInternalCh, errInternalCh, packageName, packageSection, indexURL)
	}

	// Run consumer worker.
	go func() {
		for errInternalCh != nil || packagesInternalCh != nil {
			select {
			case p, ok := <-packagesInternalCh:
				if ok {
					log.Info("getIndexPackages: get response from DB")
					if len(p) > 0 {
						packagesCh <- p
					}
					continue
				}
				packagesInternalCh = nil
			case e, ok := <-errInternalCh:
				if ok {
					log.Info("getIndexPackages: get error from DB")
					errCh <- e
					continue
				}
				errInternalCh = nil
			}
		}
		done <- true
	}()

	// Wait for producers to complete.
	indexesWG.Wait()
	close(packagesInternalCh)
	close(errInternalCh)

	// Wait for consumers to complete.
	<-done
}

func getIndexPackages(progressBar *progressbar.ProgressBar, waitGroup *sync.WaitGroup, packagesCh chan []archive.Package, errCh chan error, packageName string, packageSection string, indexURL string) {
	defer waitGroup.Done()
	defer progressBar.Add(1)

	log.WithField("URL", indexURL).Debug("Downloading compressed index file")

	resp, err := http.Get(indexURL)
	defer resp.Body.Close()
	if err != nil {
		errCh <- err
		return
	}

	if got, want := resp.StatusCode, http.StatusOK; got != want {
		errCh <- fmt.Errorf("download(%s): unexpected HTTP status code: got %d, want %d", indexURL, got, want)
		return
	}

	log.WithField("URL", indexURL).Debug("Decompressing index file")

	debDecompressor := deb.DecompressorFor(PACKAGES_INDEX_FORMAT)
	rd, err := debDecompressor(resp.Body)
	defer rd.Close()
	if err != nil {
		errCh <- err
		return
	}

	log.WithField("URL", indexURL).Debug("Loading packages DB from index file")

	db, err := archive.LoadPackages(rd)
	if err != nil {
		errCh <- err
		return
	}

	log.WithField("URL", indexURL).Debug("Querying packages from DB")

	p, err := db.Map(func(p *archive.Package) bool {
		if strings.Contains(p.Package, packageName) && p.Section == packageSection && p.Architecture.CPU != "all" {
			return true
		}
		return false
	})
	if err != nil {
		errCh <- err
		return
	}

	packagesCh <- p
}

func getInReleaseFromDistURL(distURL string) (*archive.Release, error) {
	inReleaseURL, err := url.JoinPath(distURL, INRELEASE)
	if err != nil {
		return nil, err
	}

	inReleaseResp, err := http.Get(inReleaseURL)
	if err != nil {
		return nil, err
	}
	if got, want := inReleaseResp.StatusCode, http.StatusOK; got != want {
		if inReleaseResp.StatusCode == 404 {
			return nil, fmt.Errorf("InRelease file not found")
		}
		if inReleaseResp.StatusCode >= 500 && inReleaseResp.StatusCode < 600 {
			return nil, fmt.Errorf("internal error from mirror for release file")
		}

		return nil, fmt.Errorf("download(%s): unexpected HTTP status code: got %d, want %d", inReleaseURL, got, want)
	}

	release, err := archive.LoadInRelease(inReleaseResp.Body, nil)
	if err != nil {
		return nil, err
	}

	return release, nil
}
