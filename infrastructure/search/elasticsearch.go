package search

import (
	"gojo/config"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
)

// EsClient 全局的 ES 客户端
var EsClient *elasticsearch.Client

// InitElasticsearch 初始化 ES 连接
func InitElasticsearch() {
	// 极其清爽的配置，因为我们在 Docker 里关了密码验证
	cfg := elasticsearch.Config{
		//Addresses: []string{
		//	"http://localhost:9200", // ES 的默认地址
		//},
		Addresses: config.GlobalConfig.Elasticsearch.Addresses,
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("❌ 致命错误：无法创建 ES 客户端: %s", err)
	}

	// Ping 一下，确保网络是通的
	res, err := client.Info()
	if err != nil {
		log.Fatalf("❌ 致命错误：连不上 ES 引擎: %s", err)
	}
	defer res.Body.Close()

	log.Printf("🚀 Elasticsearch 连接成功！集群信息: %s\n", res.Status())
	EsClient = client
}
