package main

import (
	"flag"
	"fmt"
	"os"

	"spider-go/crawler"
)

func main() {
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	config, err := crawler.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	c, err := crawler.NewBiliCrawler(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化爬虫失败: %v\n", err)
		os.Exit(1)
	}

	c.Run()
}
