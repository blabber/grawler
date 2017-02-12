package grawler

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

var itemTypeStringTests = []struct {
	itemType ItemType
	expected string
}{
	{DirectoryType, "1"},
	{ItemType('2'), "2"},
	{ItemType('g'), "g"},
}

func TestItemTypeString(t *testing.T) {
	for _, tt := range itemTypeStringTests {
		s := tt.itemType.String()
		if s != tt.expected {
			t.Errorf("%q != %q", s, tt.expected)
		}
	}
}

var hostStringTests = []struct {
	host     *Host
	expected string
}{
	{&Host{"gopher.floodgap.com", "70"}, "gopher.floodgap.com:70"},
	{&Host{"gopher.floodgap.com", "72"}, "gopher.floodgap.com:72"},
	{&Host{"Gopher.Floodgap.Com", "72"}, "gopher.floodgap.com:72"},
	{&Host{"gopher.raumzeitlabor.org", "70"}, "gopher.raumzeitlabor.org:70"},
	{&Host{"127.0.0.1", "70"}, "127.0.0.1:70"},
	{&Host{"::1", "70"}, "[::1]:70"},
}

func TestHostString(t *testing.T) {
	for _, tt := range hostStringTests {
		s := tt.host.String()
		if s != tt.expected {
			t.Errorf("%q != %q", s, tt.expected)
		}
	}
}

var newResourceFromGopherLineTests = []struct {
	line     string
	expected string
	invalid  bool
}{
	// Valid gopher lines
	{"1Root\t\tlocalhost\t70", "gopher://localhost:70/1", false},
	{"1Root\t/\tlocalhost\t70", "gopher://localhost:70/1", false},
	{"1A directory\t/Dir/Selector\tlocalhost\t70", "gopher://localhost:70/1/Dir/Selector", false},
	{"1A directory\t/Dir/Selector\tlocalhost\t70\t+", "gopher://localhost:70/1/Dir/Selector", false},
	{"2File\tFileselector\tgopher.example.com\t72", "gopher://gopher.example.com:72/2Fileselector", false},
	{"2File\tFile selector\tgopher.example.com\t72", "gopher://gopher.example.com:72/2File%20selector", false},
	// Malformed gopher lines
	{"1Too many fields\t/Directory/Selector\tlocalhost\t70\tDerp", "", true},
	{"1Too many fields\t/Directory/Selector\tlocalhost\t70\tHerp\tDerp", "", true},
	{"1Too few fields\t/Directory/Selector\tlocalhost", "", true},
	{"\t/Directory/Selector\tlocalhost\t70", "", true},
	{"1Missing host\t/Directory/Selector\t\t70", "", true},
	{"1Missing port\t/Directory/Selector\tlocalhost\t", "", true},
	{"1Space in host\tSome selector\tgopher.example.com 72\t72", "", true},
	{"1Space in port\tSome selector\tgopher.example.com\t7 2", "", true},
	{"1Space as host\tSome selector\t \t72", "", true},
	{"1Space as port\tSome selector\tgopher.example.com\t ", "", true},
	{"1Slash in host\tSome selector\tgopher.example.com/dir\t70", "", true},
}

func TestNewResourceFromGopherLine(t *testing.T) {
	for _, tt := range newResourceFromGopherLineTests {
		r, err := NewResourceFromGopherLine(tt.line)
		switch {
		case err != nil && !tt.invalid:
			t.Errorf("Parsing %q failed unexpected", tt.line)
		case err == nil && tt.invalid:
			t.Errorf("Parsing %q succeeded unexpected", tt.line)
		}

		if r == nil {
			continue
		}

		s := r.String()
		if s != tt.expected {
			t.Errorf("%q != %q", s, tt.expected)
		}
	}
}

var resourceStringTests = []struct {
	resource *Resource
	expected string
}{
	{&Resource{&Host{"localhost", "70"}, ItemType('1'), ""}, "gopher://localhost:70/1"},
	{&Resource{&Host{"localhost", "70"}, ItemType('1'), "/"}, "gopher://localhost:70/1"},
	{&Resource{&Host{"example.com", "72"}, ItemType('g'), "/Test"}, "gopher://example.com:72/g/Test"},
}

func TestResourceString(t *testing.T) {
	for _, tt := range resourceStringTests {
		s := tt.resource.String()
		if s != tt.expected {
			t.Errorf("%q != %q", s, tt.expected)
		}
	}
}

var crawlFindingStringTests = []struct {
	finding  *CrawlFinding
	expected string
}{
	{
		&CrawlFinding{
			&Resource{&Host{"referenced", "72"}, ItemType('1'), ""},
			&Host{"parent", "70"},
		}, `"parent:70" -> "referenced:72"`,
	},
	{
		&CrawlFinding{
			&Resource{&Host{"localhost", "70"}, ItemType('2'), "/Test"},
			&Host{"gopher.example.com", "72"},
		}, `"gopher.example.com:72" -> "localhost:70"`,
	},
	{
		&CrawlFinding{
			&Resource{&Host{"gopher.example.com", "72"}, ItemType('2'), "/Test"},
			&Host{"gopher.example.com", "72"},
		}, `"gopher.example.com:72" -> "gopher.example.com:72"`,
	},
	{
		&CrawlFinding{
			&Resource{&Host{"gopher.example.com", "72"}, ItemType('2'), "/Test"},
			nil,
		}, `"gopher.example.com:72"`,
	},
}

func TestCrawlFindingString(t *testing.T) {
	for _, tt := range crawlFindingStringTests {
		s := tt.finding.String()
		if s != tt.expected {
			t.Errorf("%q != %q", s, tt.expected)
		}
	}
}

type stringReadCloser struct {
	*strings.Reader
}

func newStringReadCloser(s string) *stringReadCloser {
	return &stringReadCloser{strings.NewReader(s)}
}

func (s *stringReadCloser) Close() error {
	return nil
}

func mockResourceOpener(r *Resource) (io.ReadCloser, error) {
	s := "iThis is a test\t\terror.host\t1\r\n"
	s += "i\t\terror.host\t1\r\n"
	s += "1Give me what you got\t\tlocalhost\t70\r\n"
	s += "2Afile\tfile\tlocalhost\t70\r\n"
	s += "1Another directory\t/directory\texample.com\t72\r\n"
	s += "."

	return newStringReadCloser(s), nil
}

func TestResourceCrawler(t *testing.T) {
	findingSet := make(map[string]bool)
	findings := make(chan *CrawlFinding)
	wait := make(chan bool)

	go func() {
		for f := range findings {
			s := fmt.Sprintf("%v->%v", f.Parent, f.Resource)
			findingSet[s] = true
		}
		wait <- true
	}()

	r := &Resource{&Host{"example.com", "70"}, '2', "/"}
	err := ResourceCrawler(mockResourceOpener, r, findings)
	if err == nil {
		t.Errorf("Resource is not a directory but no error occured: %v", r)
	}

	r = &Resource{&Host{"localhost", "70"}, DirectoryType, "/"}
	err = ResourceCrawler(mockResourceOpener, r, findings)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	close(findings)
	<-wait

	expected := []string{
		fmt.Sprintf("%v->%s", r.Host, "gopher://localhost:70/1"),
		fmt.Sprintf("%v->%s", r.Host, "gopher://example.com:72/1/directory"),
	}

	for _, e := range expected {
		if !findingSet[e] {
			t.Errorf("Relation %q not found in %#v", e, findingSet)
		}
	}

	if len(findingSet) > len(expected) {
		t.Fatalf("Too many findings: %v", findingSet)
	}
}

var coordinatorTests = []*Resource{
	&Resource{&Host{"example.com", "70"}, '1', "/test"},
	&Resource{&Host{"localhost", "7070"}, '0', "/dummy.txt"},
}

func TestCoordinatorQueueJob1(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("Coordinator could not be created.")
	}

	for _, ct := range coordinatorTests {
		err := c.QueueJob(ct)
		if err != nil {
			t.Fatalf("Unexpected error while queuing job: %v", err)
		}
	}

	if len(c.queued) != len(coordinatorTests) {
		t.Fatalf("Number of queued jobs unexpected: %d != %d",
			len(c.queued), len(coordinatorTests))
	}

	for _, ct := range coordinatorTests {
		if _, ok := c.queued[ct.String()]; !ok {
			t.Errorf("Job %v not found in queued jobs: %#v", ct, c.queued)
		}
	}

	for _, ct := range coordinatorTests {
		err := c.QueueJob(ct)
		if err == nil {
			t.Fatalf("No error while requeuing job: %v", ct)
		}
	}

	if len(c.queued) != len(coordinatorTests) {
		t.Fatalf("Number of queued jobs unexpected (after readding): %d != %d",
			len(c.queued), len(coordinatorTests))
	}
}

func TestCoordinatorQueueJob2(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("Coordinator could not be created.")
	}

	for _, ct := range coordinatorTests {
		c.active[ct.String()] = true
	}

	for _, ct := range coordinatorTests {
		err := c.QueueJob(ct)
		if err == nil {
			t.Fatalf("No error while requeuing active job: %v", ct)
		}
	}

	if len(c.queued) != 0 {
		t.Fatalf("Number of queued jobs unexpected: %d != 0", len(c.queued))
	}
}

func TestCoordinatorQueueJob3(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("Coordinator could not be created.")
	}

	for _, ct := range coordinatorTests {
		c.finished[ct.String()] = true
	}

	for _, ct := range coordinatorTests {
		err := c.QueueJob(ct)
		if err == nil {
			t.Fatalf("No error while requeuing finished job: %v", ct)
		}
	}

	if len(c.queued) != 0 {
		t.Fatalf("Number of queued jobs unexpected: %d != 0", len(c.queued))
	}
}

func TestCoordinatorQueuedJob(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("Coordinator could not be created.")
	}

	for _, ct := range coordinatorTests {
		c.queued[ct.String()] = ct
	}

	results := []*Resource{}
	for i := range coordinatorTests {
		q := c.QueuedJob()
		if q == nil {
			t.Fatalf("Queued job #%d is unexpectedly nil.", i)
			continue
		}
		results = append(results, q)
	}
	if q := c.QueuedJob(); q != nil {
		t.Fatalf("Unexpected queued job: %v", q)
	}

	if len(c.queued) > 0 {
		t.Fatalf("Leftover queued job(s): %d != 0", len(c.queued))
	}

	if len(c.active) != len(coordinatorTests) {
		t.Fatalf("Number of active jobs unexpected: %d != %d", len(c.active), len(coordinatorTests))
	}

	if len(results) != len(coordinatorTests) {
		t.Fatalf("Number of jobs retrieved unexpected: %d != %d",
			len(results), len(coordinatorTests))
	}

	for _, ct := range coordinatorTests {
		if _, ok := c.active[ct.String()]; !ok {
			t.Errorf("Job %v not found in active jobs: %#v", ct, c.active)
		}
	}

	for _, ct := range coordinatorTests {
		found := false
		for _, r := range results {
			if ct.String() == r.String() {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Job not retrieved from queue: %v", ct)
		}
	}
}

func TestCoordinatorFinishJob(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("Coordinator could not be created.")
	}

	for _, ct := range coordinatorTests {
		c.active[ct.String()] = true
	}

	for _, ct := range coordinatorTests {
		c.FinishJob(ct)
	}

	if len(c.active) > 0 {
		t.Fatalf("Number of active jobs unexpected: %d != 0", len(c.active))
	}

	if len(c.finished) > len(coordinatorTests) {
		t.Fatalf("Number of finished jobs unexpected: %d != %d", len(c.active), len(coordinatorTests))
	}

	for _, ct := range coordinatorTests {
		if _, ok := c.finished[ct.String()]; !ok {
			t.Errorf("Job %v not found in finished jobs: %#v", ct, c.finished)
		}
	}
}

func TestCoordinatorJobsExhausted(t *testing.T) {
	c := NewCoordinator()
	if c == nil {
		t.Fatal("Coordinator could not be created.")
	}

	if c.JobsExhausted() {
		t.Fatal("Jobs unexpectedly exhausted.")
	}

	for _, ct := range coordinatorTests {
		if c.JobsExhausted() {
			t.Fatal("Jobs unexpectedly exhausted.")
		}
		c.QueueJob(ct)
	}

	if c.JobsExhausted() {
		t.Fatal("Jobs unexpectedly exhausted.")
	}

	for range coordinatorTests {
		if c.JobsExhausted() {
			t.Fatal("Jobs unexpectedly exhausted.")
		}
		c.QueuedJob()
	}

	if c.JobsExhausted() {
		t.Fatal("Jobs unexpectedly exhausted.")
	}

	for _, ct := range coordinatorTests {
		if c.JobsExhausted() {
			t.Fatal("Jobs unexpectedly exhausted.")
		}
		c.FinishJob(ct)
	}

	if !c.JobsExhausted() {
		t.Fatal("Jobs unexpectedly not exhausted.")
	}
}

type mockDotfile struct {
	bytes.Buffer
}

func (*mockDotfile) Close() error {
	return nil
}

var emptyDotfile = "strict digraph {\n}\n"
var nonEmptyDotfile = `strict digraph {
	"parent:70"[alive=true]
	"parent:70" -> "referenced:72"
	"gopher.example.com:72"[alive=true]
	"gopher.example.com:72" -> "localhost:70"
	"gopher.example.com:72" -> "gopher.example.com:72"
}
`

func TestGrapherClose(t *testing.T) {
	f := new(mockDotfile)
	g, err := NewGrapher(f)
	if err != nil {
		t.Fatal("NewGrapher failed.")
	}
	g.Close()

	if emptyDotfile != f.String() {
		t.Fatalf("Unexpected dotfile content: %q != %q", emptyDotfile, f.String())
	}
}

func TestGrapherGraphFinding(t *testing.T) {
	f := new(mockDotfile)
	g, err := NewGrapher(f)
	if err != nil {
		t.Fatal("NewGrapher failed.")
	}
	for _, tt := range crawlFindingStringTests {
		g.GraphFinding(tt.finding)
	}
	g.Close()

	if nonEmptyDotfile != f.String() {
		t.Fatalf("Unexpected dotfile content: %q != %q", nonEmptyDotfile, f.String())
	}
}
