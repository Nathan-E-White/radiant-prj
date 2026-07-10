//go:build dataplane

package gateway

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type kafkaBrokerReader struct {
	reader *kafka.Reader
}

func newKafkaBrokerReader(brokers []string, topic string, groupID string) SimopsKafkaReader {
	return &kafkaBrokerReader{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			MinBytes:    1,
			MaxBytes:    10e6,
			StartOffset: kafka.FirstOffset,
		}),
	}
}

func (r *kafkaBrokerReader) FetchMessage(ctx context.Context) (SimopsBrokerMessage, error) {
	msg, err := r.reader.FetchMessage(ctx)
	if err != nil {
		return SimopsBrokerMessage{}, err
	}
	return fromKafkaMessage(msg), nil
}

func (r *kafkaBrokerReader) CommitMessages(ctx context.Context, msgs ...SimopsBrokerMessage) error {
	kafkaMessages := make([]kafka.Message, 0, len(msgs))
	for _, msg := range msgs {
		kafkaMessages = append(kafkaMessages, toKafkaMessage(msg))
	}
	return r.reader.CommitMessages(ctx, kafkaMessages...)
}

func (r *kafkaBrokerReader) Close() error {
	return r.reader.Close()
}

type kafkaBrokerWriter struct {
	writer *kafka.Writer
}

func newKafkaBrokerWriter(brokers []string, topic string) simopsBrokerWriter {
	return &kafkaBrokerWriter{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		},
	}
}

func (w *kafkaBrokerWriter) WriteMessages(ctx context.Context, msgs ...SimopsBrokerMessage) error {
	kafkaMessages := make([]kafka.Message, 0, len(msgs))
	for _, msg := range msgs {
		kafkaMessages = append(kafkaMessages, toKafkaMessage(msg))
	}
	return w.writer.WriteMessages(ctx, kafkaMessages...)
}

func fromKafkaMessage(msg kafka.Message) SimopsBrokerMessage {
	return SimopsBrokerMessage{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       append([]byte(nil), msg.Key...),
		Value:     append([]byte(nil), msg.Value...),
		Time:      msg.Time,
	}
}

func toKafkaMessage(msg SimopsBrokerMessage) kafka.Message {
	return kafka.Message{
		Topic:     msg.Topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
		Key:       append([]byte(nil), msg.Key...),
		Value:     append([]byte(nil), msg.Value...),
		Time:      msg.Time,
	}
}
