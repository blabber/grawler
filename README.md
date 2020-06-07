grawler
=======

A gopherspace crawler
---------------------

### Project status

This project is not maintained any more. If you are interested in it, ping me
and I will transfer maintainership/ownership of this project to you.

In the current state this software should not be used, as it is not a well
behaving crawler:

* It ignores any `robots.txt` that gopher holes might be providing to restrict
  crawling
* It also does not attempt to reduce the load on the gopher servers, which
  often run on restricted resources, by spreading the requests over time

### What is it?

`grawler` is a gopherspace crawler crawling all servers reachable (direct or
indirect) from a gopherhole used as starting point. By default `grawler` will
start crawling with gopher.floodgap.com, probably the most central gopherhole in
existence.

To crawl the whole gopherspace, `grawler` will need a few hours on a reasonable
fast computer and internet connection - todays gopherspace is not as big as it
used to be :(

### Installation

`grawler` is written in go. So you need a go environment and can install
`grawler` by calling

	go install github.com/blabber/grawler

### Let the grawling begin

To start the crawler just call `grawler`. It will issue log messages on stderr
and generate a file called `grawler.dot` that can be postprocessed using the
[graphviz](http://www.graphviz.org) graph visualization software.

### Results

You can find an example `grawler.dot` in the [results](./results) folder. If you
do something cool with this data or your own result sets, please let me know.

#### Statistics

	Gopherholes
	  alive:  228
	  dead:   162
	  total:  390
	
	Top 5 TLDs for alive gopherholes
	  .org   73
	  .net   41
	  .com   36
	  .de    9
	  .uk    5
	
	Top 5 TLDs for dead gopherholes
	  .org   33
	  .net   21
	  .hu    18
	  .edu   18
	  .com   13

#### Some graphs

##### Raw graph

This graph was generated using the following commands:

	sfdp -Tsvg -o graphs/raw.svg results/grawler.dot

<a href="./graphs/raw.svg" target="_blank">
	<img src="./graphs/raw_thumb.png"/>
</a>

##### All identified gopherholes, unresponsive ones colored red

This graph was generated using the following commands:

	gvpr -f tools/colorize.g results/grawler.dot | \
		sfdp -Tsvg -Goverlap=false -Gsplines=true -o graphs/colored.svg

<a href="./graphs/colored.svg" target="_blank">
	<img src="./graphs/colored_thumb.png"/>
</a>

##### All responsive gopherholes

This graph was generated using the following commands:

	gvpr -f tools/cleanup.g results/grawler.dot | \
		sfdp -Tsvg -Goverlap=false -Gsplines=true -o graphs/alive.svg

<a href="./graphs/alive.svg" target="_blank">
	<img src="./graphs/alive_thumb.png"/>
</a>

