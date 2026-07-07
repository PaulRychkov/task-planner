package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

type Producer struct {
	mu       sync.Mutex
	brokers  []string
	topic    string
	log      *zap.Logger
	producer sarama.SyncProducer
}

func NewProducer(brokers []string, topic string, log *zap.Logger) *Producer {
	return &Producer{brokers: brokers, topic: topic, log: log}
}

func (p *Producer) connectLocked() error {
	if p.producer != nil {
		return nil
	}
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 3
	cfg.Net.DialTimeout = 3 * time.Second
	producer, err := sarama.NewSyncProducer(p.brokers, cfg)
	if err != nil {
		return fmt.Errorf("connect kafka %v: %w", p.brokers, err)
	}
	p.producer = producer
	p.log.Info("kafka producer connected", zap.Strings("brokers", p.brokers), zap.String("topic", p.topic))
	return nil
}

func (p *Producer) Publish(ctx context.Context, key string, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.connectLocked(); err != nil {
		return err
	}
	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	if _, _, err := p.producer.SendMessage(msg); err != nil {
		if closeErr := p.producer.Close(); closeErr != nil {
			p.log.Warn("close kafka producer after send failure", zap.Error(closeErr))
		}
		p.producer = nil
		return fmt.Errorf("send to %s: %w", p.topic, err)
	}
	return nil
}

func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.producer == nil {
		return nil
	}
	err := p.producer.Close()
	p.producer = nil
	if err != nil {
		return fmt.Errorf("close kafka producer: %w", err)
	}
	return nil
}
