package clusternotification

import (
	"context"
	"log"
	"os"
	_ "os/exec"
	_ "strings"
	"testing"
	_ "time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
)

func TestHelloPubSubSystem(t *testing.T) {
	ctx := context.Background()

	topicName := os.Getenv("FUNCTIONS_TOPIC")
	projectID := os.Getenv("GCP_PROJECT")

	//startTime := time.Now().UTC().Format(time.RFC3339)

	// Create the Pub/Sub client and topic.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatal(err)
	}
	topic := client.Topic(topicName)

	// Publish a message with a random string to verify.
	// We use a random string to make sure the function is logging the correct
	// message for this test invocation.
	u := uuid.New()
	msg := &pubsub.Message{
		Data: []byte(u.String()),
	}
	topic.Publish(ctx, msg).Get(ctx)
}
