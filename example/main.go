package main

import (
	"flag"
	"fmt"
	"net/http/cookiejar"
	"os"

	urltomd "go.kvsh.ch/url-to-md"
	"go.uber.org/zap"
)

func main() {
	verbose := flag.Bool("v", false, "verbose logging")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: example [-v] <url>")
		os.Exit(1)
	}

	var opts []urltomd.Option
	opts = append(opts, urltomd.WithUserAgent("MyReader/1.0"))

	if *verbose {
		logger, _ := zap.NewDevelopment()
		opts = append(opts, urltomd.WithLogger(logger))
	}

	jar, _ := cookiejar.New(nil)
	opts = append(opts, urltomd.WithCookieJar(jar))

	article, err := urltomd.Convert(args[0], opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("Title:   %s\n", article.Title)
	fmt.Printf("Byline:  %s\n", article.Byline)
	fmt.Printf("Excerpt: %s\n\n", article.Excerpt)
	fmt.Println(article.Content)
}
