package consumer

import (
	"git.code.oa.com/trpc-go/trpc-database/kafka"
)

// kafkaTask kafka任务队列
var kafkaTask = map[string]kafka.KafkaConsumer{
	"contentLog":       NewContentLogConsumer(),
	"contentProcessor": NewContentProcessorConsumer(),
	"contentNotify":    NewContentNotifyConsumer(),
	"crawlHydra":       NewCrawlHydraConsumer(),
	"kafkaProduce":     NewKafkaProduce(),
}

// GetKafkaList 获取kafka消费列表
func GetKafkaList() map[string]kafka.KafkaConsumer {
	return kafkaTask
}
