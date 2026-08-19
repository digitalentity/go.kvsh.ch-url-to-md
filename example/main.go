package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http/cookiejar"
	"os"
	"time"

	urltomd "go.kvsh.ch/url-to-md"
	"go.uber.org/zap"
)

func main() {
	verbose := flag.Bool("v", false, "verbose logging")
	timeout := flag.Duration("t", 30*time.Second, "request timeout")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: example [-v] [-t timeout] <url>")
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	article, err := urltomd.ConvertContext(ctx, args[0], opts...)
	if err != nil {
		if errors.Is(err, urltomd.ErrChallengeBlocked) {
			fmt.Fprintf(os.Stderr, "blocked by anti-bot challenge: %v (retryable: %v)\n", err, urltomd.Retryable(err))
		} else if errors.Is(err, urltomd.ErrRateLimited) {
			fmt.Fprintf(os.Stderr, "rate limited: %v (retryable: %v)\n", err, urltomd.Retryable(err))
		} else {
			fmt.Fprintf(os.Stderr, "error: %v (retryable: %v)\n", err, urltomd.Retryable(err))
		}
		os.Exit(1)
	}

	fmt.Printf("Title:       %s\n", article.Title)
	fmt.Printf("Byline:      %s\n", article.Byline)
	fmt.Printf("Excerpt:     %s\n", article.Excerpt)
	fmt.Printf("Language:    %s\n", article.Language)
	fmt.Printf("Source:      %s\n", article.Source)
	fmt.Printf("IsTruncated: %v\n", article.IsTruncated)
	if article.PublishedTime != nil {
		fmt.Printf("Published:   %s\n", article.PublishedTime)
	}
	fmt.Println()
	fmt.Println(article.Content)
}
