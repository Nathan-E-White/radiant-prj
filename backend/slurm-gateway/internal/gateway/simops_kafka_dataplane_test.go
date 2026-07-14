//go:build dataplane

package gateway

import (
	"reflect"
	"testing"
)

func TestKafkaBrokerMessagePreservesOwnedHeaders(t *testing.T) {
	original := SimopsBrokerMessage{
		Topic: "digital-twin.state.v1",
		Key:   []byte("TWIN-1"),
		Value: []byte(`{"schemaVersion":"digital-twin.state.v1"}`),
		Headers: []SimopsBrokerHeader{{
			Key:   workbenchTwinLineageHeader,
			Value: []byte(`[{"lineageId":"LIN-1"}]`),
		}},
	}
	kafkaMessage := toKafkaMessage(original)
	original.Headers[0].Value[0] = 'X'
	roundTrip := fromKafkaMessage(kafkaMessage)
	if !reflect.DeepEqual(roundTrip.Headers, []SimopsBrokerHeader{{Key: workbenchTwinLineageHeader, Value: []byte(`[{"lineageId":"LIN-1"}]`)}}) {
		t.Fatalf("Kafka header round trip=%#v", roundTrip.Headers)
	}
	kafkaMessage.Headers[0].Value[0] = 'Y'
	if roundTrip.Headers[0].Value[0] != '[' {
		t.Fatalf("round-trip header aliases Kafka message: %q", roundTrip.Headers[0].Value)
	}
}
