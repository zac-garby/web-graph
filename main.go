package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/html"

	gv "github.com/awalterschulze/gographviz"
)

type edge struct {
	from, to string
}

var (
	graph *gv.Graph

	// A list of URLs already handled
	handled []string

	// A list of edges already handled
	handledEdges []edge

	// The maxmimum depth allowed
	maxDepth = 4

	// The maximum number of nodes to generate
	maxNodes = 32

	// The current amount of nodes
	nodeCount = 0

	// The timeout for each GET request, in milliseconds
	timeout = 7500
)

func main() {
	url := flag.String("url", "https://golang.org", "the URL to analyse")
	flag.IntVar(&maxDepth, "depth", 4, "the maximum depth to go into the tree")
	flag.IntVar(&maxNodes, "nodes", 32, "the maximum amount of nodes to generate")
	flag.IntVar(&timeout, "timeout", 7500, "the timeout on the GET requests")
	flag.Parse()

	graph = gv.NewGraph()
	graph.SetDir(true)
	graph.Name = "web"
	graph.AddAttr("web", "rankdir", "LR")

	if err := analyseURL(*url, 0); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}

	if err := generateImage(); err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(2)
	}
}

func analyseURL(u string, depth int) error {
	if nodeCount >= maxNodes || depth > maxDepth {
		return nil
	}

	for _, h := range handled {
		if h == u {
			return nil
		}
	}

	nodeCount++
	fmt.Printf(strings.Repeat(" ", depth)+"% 5d: %s\n", nodeCount, u)

	handled = append(handled, u)

	if err := graph.AddNode("web", quote(u), nil); err != nil {
		return err
	}

	client := http.Client{
		Timeout: time.Millisecond * time.Duration(timeout),
	}

	resp, err := client.Get(u)
	if err != nil {
		if err == http.ErrServerClosed {
			return nil
		}

		return err
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}

	base, err := url.Parse(u)
	if err != nil {
		return err
	}

	var walk func(*html.Node) error

	walk = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					parsed, err := url.Parse(attr.Val)
					if err != nil {
						return err
					}

					attr.Val = base.ResolveReference(parsed).String()

					if err := analyseURL(attr.Val, depth+1); err != nil {
						return err
					}

					connect(quote(attr.Val), quote(u))
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}

		return nil
	}

	return walk(doc)
}

func connect(from, to string) {
	if graph.IsNode(from) {
		for _, e := range handledEdges {
			if e.from == from && e.to == to {
				return
			}
		}

		handledEdges = append(handledEdges, edge{
			from: from,
			to:   to,
		})

		graph.AddEdge(from, to, true, map[string]string{
			"dir": "reversed",
		})
	}
}

func quote(s string) string {
	return fmt.Sprintf(`"%s"`, s)
}

func generateImage() error {
	fmt.Println("generating image...")

	cmd := exec.Command("dot", "-Tsvg", "-oout.svg")
	cmd.Stdin = strings.NewReader(graph.String())

	out, err := cmd.Output()
	if err != nil {
		return err
	}

	fmt.Println(string(out))

	return nil
}
