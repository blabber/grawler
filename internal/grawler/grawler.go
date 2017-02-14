// "THE BEER-WARE LICENSE" (Revision 42):
// <tobias.rehbein@web.de> wrote this file. As long as you retain this notice
// you can do whatever you want with this stuff. If we meet some day, and you
// think this stuff is worth it, you can buy me a beer in return.
//                                                             Tobias Rehbein

// Building blocks for the grawler gopherspace crawler.
package grawler

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

// ItemType is the type of an item in a gopher menu.
type ItemType byte

// DirectoryType is the ItemType to describe a directory.
const DirectoryType ItemType = '1'
const InformationalMessageType ItemType = 'i'
const ErrorMessageType ItemType = '3'

// String returns the string representation of the ItemType
func (t ItemType) String() string {
	return string(t)
}

// Token is the type describing a field position of a token in a gopher menu
// line.
type Token int

// Tokens of a valid gopher menu line.
const (
	TTypeAndDescription Token = iota // ItemType and descriptive text
	TSelector                        // Selector string
	THostname                        // Host
	TPort                            // Port
	TPlus                            // Additional field used by Gopher+
)

// Host describes a gopher server.
type Host struct {
	Hostname string
	Port     string
}

// String returns the string representation of a gopher server suitable for
// net.Dial() calls. As the returned string is used as a key in maps all
// characters are converted to lower case.
func (h *Host) String() string {
	return strings.ToLower(net.JoinHostPort(h.Hostname, h.Port))
}

// Resource represents a gopher resource identified by a *Host, ItemType and
// selector string.
type Resource struct {
	*Host
	Type     ItemType
	Selector string
}

// NewResourceFromGopherLine parses a gopher menu line an returns a
// corresponding *Resource.
func NewResourceFromGopherLine(line string) (*Resource, error) {
	t := strings.Split(line, "\t")

	valid := true
	switch {
	case len(t) < 4 || len(t) > 5:
		// at least four fields are required for a valid gopher line, at
		// most five fields are allowed for a valid gopher+ line
		valid = false
	case len(t[THostname]) == 0 || len(t[TPort]) == 0:
		// host and port must not be empty
		valid = false
	case strings.Contains(t[THostname], " ") || strings.Contains(t[TPort], " "):
		// host and port must not contain spaces
		valid = false
	case strings.Contains(t[THostname], "/"):
		// host and port must not contain slashes
		valid = false
	case len(t) == 5 && t[TPlus] != "+":
		// for a valid gopher+ line the fifth fields has to be "+"
		valid = false
	case len(t[TTypeAndDescription]) == 0:
		// an ItemType is required (first byte of TTypeAndDescription)
		valid = false
	}
	if !valid {
		err := fmt.Errorf("Could not parse Gopher line to resource: %q", line)
		return nil, err
	}

	return &Resource{
		&Host{t[THostname], t[TPort]},
		ItemType(t[TTypeAndDescription][0]),
		t[TSelector],
	}, nil
}

// String returns a URI string representation of a Resource. If the selector
// string is "/" it is replaced by an empty string.
func (r *Resource) String() string {
	s := r.Selector
	if s == "/" {
		s = ""
	}
	u, _ := url.Parse(fmt.Sprintf("gopher://%v/%v%s", r.Host, r.Type, s))
	return u.String()
}

// CrawlFinding represents a reference to another Resource found by a crawler,
// identified by the referenced Resource and the Parent Host referencing this
// Resource.
type CrawlFinding struct {
	Resource *Resource
	Parent   *Host
}

// String returns a string representation suitable for inclusion in a dot file.
// It represents a directed edge between parent Host and referenced Host. The
// selector string and item type are deliberately dropped.
func (f *CrawlFinding) String() string {
	if f.Parent == nil {
		return fmt.Sprintf(`"%v"`, f.Resource.Host)
	}

	return fmt.Sprintf(`"%v" -> "%v"`, f.Parent, f.Resource.Host)
}

// ResourceOpener implements a way to open a Resource, returning a io.ReadCloser
// that can be used to read the Resource. The caller is expexted to close the
// returned io.ReadCloser.
type ResourceOpener func(*Resource) (io.ReadCloser, error)

// NetResourceOpener is a ResourceOpener that implements a way to open Resource
// r via a network connection.
//
// Establishing a connection times out after five seconds. An established
// connection times out after one minute.
func NetResourceOpener(r *Resource) (io.ReadCloser, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(r.Hostname, r.Port),
		time.Second*5)
	if err != nil {
		return nil, err
	}

	err = conn.SetDeadline(time.Now().Add(time.Minute))
	if err != nil {
		conn.Close()
		return nil, err
	}

	_, err = fmt.Fprintf(conn, "%s\r\n", r.Selector)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// ItemActionFunc is a function that can be called by ResourceCrawler for items
// that are neither informational texts nor error messages.
type ItemActionFunc func(Resource)

// ResourceCrawler crawls a gopher menz. It uses ResourceOpener o to get access
// to a a Resource r that is expected to be of type DirectoryType. It then looks
// for references to other directories and reports its findings via the out
// channel.
//
// If ItemAction are passed, they are called for every Resource in the
// directory that is not a InformationalMessageType or ErrorMessageType.
func ResourceCrawler(o ResourceOpener, r *Resource, out chan<- *CrawlFinding, ia ...ItemActionFunc) error {
	if r.Type != DirectoryType {
		return fmt.Errorf("Resource is not a directory: %v", r)
	}

	rc, err := o(r)
	if err != nil {
		return err
	}
	defer rc.Close()

	scan := bufio.NewScanner(rc)
	for scan.Scan() {
		if len(scan.Bytes()) == 1 && scan.Bytes()[0] == '.' {
			// This is the end marker of the directory listing.
			break
		}

		res, err := NewResourceFromGopherLine(scan.Text())
		if err != nil {
			return err
		}

		if res.Type != InformationalMessageType && res.Type != ErrorMessageType {
			for _, a := range ia {
				a(*res)
			}
		}

		if res.Type == DirectoryType {
			// Yep, it is a directory item
			res, err := NewResourceFromGopherLine(scan.Text())
			if err != nil {
				return err
			}
			f := &CrawlFinding{res, r.Host}
			out <- f
		}
	}
	if err = scan.Err(); err != nil {
		return err
	}

	return nil
}

// Coordinator coordinates jobs for the crawler. Jobs can be queued, retrieved
// and marked as finished.  Coordinator tries to make sure every job is
// retrieved exactly once.
type Coordinator struct {
	queued   map[string]*Resource
	active   map[string]bool
	finished map[string]bool
}

// NewCoordinator creates and initializes a new Coordinator.
func NewCoordinator() *Coordinator {
	return &Coordinator{
		queued:   make(map[string]*Resource),
		active:   make(map[string]bool),
		finished: make(map[string]bool),
	}
}

// String returns an informative string representation. It contains the number
// of queued jobs and the number of active jobs (retrieved and not marked as
// finished).
func (c *Coordinator) String() string {
	return fmt.Sprintf("Queued:%v Active:%v Finished:%v\n", len(c.queued), len(c.active),
		len(c.finished))
}

// QueueJob queues a job to crawl *Resource r. The job is discarded if
// Coordinator already knows the job. This makes sure that no Resource is
// crawled multiple times.
//
// Hacky: An error is returned if the job has not been queued. This is
// generally not a real error condition.
func (c *Coordinator) QueueJob(r *Resource) error {
	if _, ok := c.queued[r.String()]; ok {
		return fmt.Errorf("Already queued %v", r)
	}
	if c.active[r.String()] {
		return fmt.Errorf("Already crawling %v", r)
	}
	if c.finished[r.String()] {
		return fmt.Errorf("Already crawled %v", r)
	}

	c.queued[r.String()] = r
	return nil
}

// QueuedJob retrieves a queued *Resource to crawl and marks the job as active.
// If no queued job is available, nil is returned.
func (c *Coordinator) QueuedJob() *Resource {
	for k, r := range c.queued {
		delete(c.queued, k)
		c.active[k] = true
		return r
	}
	return nil
}

// FinishJob marks *Resource r as crawled. The job has to be marked active by
// QueuedJob.
func (c *Coordinator) FinishJob(r *Resource) {
	delete(c.active, r.String())
	c.finished[r.String()] = true
}

// JobsExhausted returns true, if all jobs have been finished.
func (c *Coordinator) JobsExhausted() bool {
	// We expect at least one finished job (the job to bootstrap the
	// crawling).
	return len(c.queued) == 0 && len(c.active) == 0 && len(c.finished) != 0
}

// Grapher generates a dotfile describing the relations between gopher servers.
type Grapher struct {
	writeCloser io.WriteCloser
	alive       map[string]bool
	graphed     map[string]bool
}

// NewGrapher initializes a new Grapher and returns it. The grapher will write
// the dotfile using the io.WriteCloser writeCloser.
func NewGrapher(writeCloser io.WriteCloser) (*Grapher, error) {
	_, err := io.WriteString(writeCloser, "strict digraph {\n")
	if err != nil {
		return nil, err
	}

	return &Grapher{
		writeCloser: writeCloser,
		alive:       make(map[string]bool),
		graphed:     make(map[string]bool),
	}, nil
}

// GraphFinding generates an edge, describing a server relation defined by
// *CrawlFinding f.  Every server relation is graphed only once.
func (g *Grapher) GraphFinding(f *CrawlFinding) error {
	if f.Parent != nil {
		if p := f.Parent.String(); !g.alive[p] {
			_, err := io.WriteString(g.writeCloser, fmt.Sprintf("\t\"%s\"[alive=true]\n", p))
			if err != nil {
				return err
			}
			g.alive[p] = true
		}

		s := fmt.Sprintf("%v", f)
		if !g.graphed[s] {
			_, err := io.WriteString(g.writeCloser, fmt.Sprintf("\t%s\n", s))
			if err != nil {
				return err
			}
			g.graphed[s] = true
		}
	}
	return nil
}

// Close closes a Grapher, the data is now ready to be processed using the
// graphviz visualization toolkit.
func (g *Grapher) Close() error {
	defer g.writeCloser.Close()

	_, err := io.WriteString(g.writeCloser, "}\n")
	if err != nil {
		return err
	}
	return nil
}
