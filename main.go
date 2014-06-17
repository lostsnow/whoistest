package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"code.google.com/p/go.net/idna"
	"github.com/domainr/go-whois/whois"
)

var (
	v, quick      bool
	concurrency   int
	zones         []string
	prefixes      []string
	_, FILE, _, _ = runtime.Caller(0)
	DIR           = filepath.Dir(FILE)
)

func init() {
	flag.BoolVar(&v, "v", false, "verbose output (to stderr)")
	flag.BoolVar(&quick, "quick", false, "Only work on a subset of zones")
	flag.IntVar(&concurrency, "concurrency", 8, "Set maximum number of concurrent requests")
}

func main() {
	flag.Parse()

	if err := main1(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main1() error {
	var err error
	zones, err = readLines("zones.txt")
	if err != nil {
		return err
	}
	prefixes, err = readLines("prefixes.txt")
	if err != nil {
		return err
	}

	// Quick for debugging?
	if quick {
		fmt.Fprintf(os.Stderr, "Quick mode enabled\n")
		zones = zones[0:50]
	}

	domains := make(map[string]bool, len(zones)*len(prefixes))
	for _, zone := range zones {
		for _, prefix := range prefixes {
			domain := prefix + "." + zone
			domains[domain] = true
			req, err := whois.Resolve(domain)
			if err == nil {
				domains[req.Host] = true
			}
		}
	}

	fmt.Fprintf(os.Stderr, "Querying whois for %d domains (%d prefixes × %d zones)\n",  len(domains), len(prefixes), len(zones))

	limiter := make(chan struct{}, concurrency) // semaphore to limit concurrency
	var wg sync.WaitGroup
	for domain, _ := range domains {
		wg.Add(1)
		go func(domain string) {
			limiter <- struct{}{} // acquire semaphore
			defer func() {        // release semaphore
				<-limiter
				wg.Done()
			}()

			req, err := whois.Resolve(domain)
			if err != nil {
				return
			}

			err = os.MkdirAll(filepath.Join(DIR, "data", "responses", req.Host), os.ModePerm)
			if err != nil {
				return
			}
		}(domain)
	}
	wg.Wait()

	return nil
}

var re = regexp.MustCompile("\\s+|#.+$")

func readLines(fn string) (out []string, err error) {
	fmt.Fprintf(os.Stderr, "Reading %s\n", fn)
	buf, err := ioutil.ReadFile(filepath.Join(DIR, "data", fn))
	if err != nil {
		return
	}
	s := strings.Trim(string(buf), "\n")
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		line = re.ReplaceAllLiteralString(line, "")
		if line != "" {
			line, _ = idna.ToASCII(line)
			out = append(out, line)
		}
	}
	return
}
