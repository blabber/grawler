// "THE BEER-WARE LICENSE" (Revision 42):
// <tobias.rehbein@web.de> wrote this file. As long as you retain this notice
// you can do whatever you want with this stuff. If we meet some day, and you
// think this stuff is worth it, you can buy me a beer in return.
//                                                             Tobias Rehbein

// grawler is a gopherspace crawler written in the Go programming language.
//
// By defaults it starts crawling the gopherspace starting from
// gopher.floodgap.com and maps relations between servers.
//
// It generates a graph description suitable for postprocessing by the graphviz
// visualization toolkit.
//
// There are some commandline flags with sensible defaults available. Try the
// -h flag to get a list of these flags.
package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/blabber/grawler/internal/grawler"
)

// blacklist some selectors. Any selector containing one of these substrings
// will not be crawled. These selectors tend to belong to "interactive" games,
// yielding endless crawls.
var blacklist = []string{
	".run*",
	".cgi?",
}

// crawledJob is used in the function main to communicate the finished job and
// a crawlerID identifying the ResourceCrawler that finished the job through
// the done channel.
type crawledJob struct {
	crawlerID int
	job       *grawler.Resource
}

// mustCreateFile creates a file named name and panics if the creation fails.
func mustCreateFile(name string) *os.File {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	return f
}

func main() {
	// Parse flags
	flagBootstrap := flag.String("bootstrap", "gopher.floodgap.com", "the first server to crawl")
	flagPort := flag.String("port", "70", "the listening port of the first server to crawl")
	flagCrawlers := flag.Int("crawlers", runtime.NumCPU(), "the number of crawlers to run concurrently")
	flagDotfile := flag.String("dotfile", "grawler.dot", "the output file")
	flagLogfile := flag.String("logfile", "", "the log file (empty for stderr)")
	flag.Parse()

	// Setup logging
	if *flagLogfile != "" {
		log.SetOutput(mustCreateFile(*flagLogfile))
	}

	// Create Coordinator
	coord := grawler.NewCoordinator()

	// Initialize Grapher
	grapher, err := grawler.NewGrapher(mustCreateFile(*flagDotfile))
	if err != nil {
		panic(err)
	}
	defer func() {
		err := grapher.Close()
		if err != nil {
			panic(err)
		}
	}()

	// Create channels and seed crawlers.
	done := make(chan *crawledJob)
	findings := make(chan *grawler.CrawlFinding)
	idleCrawlers := make(chan int, *flagCrawlers)
	for i := 0; i < *flagCrawlers; i++ {
		idleCrawlers <- i + 1
	}

	ticks := time.Tick(time.Minute)

	// Bootstrap the crawling.
	go func() {
		h := *flagBootstrap
		p := *flagPort
		findings <- &grawler.CrawlFinding{
			Resource: &grawler.Resource{
				Host: &grawler.Host{
					Hostname: h,
					Port:     p,
				},
				Type:     grawler.DirectoryType,
				Selector: "",
			},
			Parent: nil}
	}()

	// Enter the main loop.
	for {
		select {
		case i := <-idleCrawlers:
			j := coord.QueuedJob()
			go func() {
				defer func() {
					done <- &crawledJob{crawlerID: i, job: j}
				}()

				if j == nil {
					return
				}

				log.Printf("[%d] Crawling %v", i, j)
				err := grawler.ResourceCrawler(grawler.NetResourceOpener, j, findings, nil)
				if err != nil {
					log.Printf("[%d] ERR: %v", i, err)
				}
				log.Printf("[%d] Done crawling %v", i, j)
			}()
		case f := <-findings:
			blacklisted := false
			for _, b := range blacklist {
				if strings.Contains(f.Resource.Selector, b) {
					blacklisted = true
					break
				}
			}
			if blacklisted {
				log.Printf("Blacklisted: %q", f.Resource.Selector)
				break
			}

			err := coord.QueueJob(f.Resource)
			if err != nil {
				log.Print(err)
			}
			err = grapher.GraphFinding(f)
			if err != nil {
				panic(err)
			}
		case j := <-done:
			if j.job != nil {
				coord.FinishJob(j.job)
			}
			idleCrawlers <- j.crawlerID
		case <-ticks:
			log.Printf("STATUS: %s", coord.String())
		}

		if coord.JobsExhausted() {
			break
		}
	}
}
