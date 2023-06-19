package clusternotification

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"

	//"os"

	//"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	//"net/smtp"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	"github.com/cloudevents/sdk-go/v2/event"
	gomail "gopkg.in/mail.v2"
)

/*
func init() {
	functions.CloudEvent("ClusterNotoficationPubSub", clusterNotoficationPubSub)
}
*/
// MessagePublishedData contains the full Pub/Sub message
// See the documentation for more details:
// https://cloud.google.com/eventarc/docs/cloudevents#pubsub

type MessagePublishedData struct {
	Message PubSubMessage
}

// PubSubMessage is the payload of a Pub/Sub event.
// See the documentation for more details:
// https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage
type PubSubMessage struct {
	Data []byte `json:"data"`
}

type email struct {
	from string
	to   string
	pass string
}

func (e email) Send(body string) error {
	msg := "From: " + e.from + "\n" + "To: " + e.to + "\n" +
		"Subject: GKE Event Notification\n\n" + body
	/*
		err := smtp.SendMail("smtp.gmail.com:587",
			smtp.PlainAuth("", e.from, e.pass, "smtp.gmail.com"),
			e.from, []string{e.to}, []byte(msg))
	*/
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", e.from)

	// Set E-Mail receivers
	m.SetHeader("To", e.to)

	// Set E-Mail subject
	m.SetHeader("Subject", "Nicklesh Test mail")

	// Set E-Mail body. You can set plain text or html with text/html
	m.SetBody("text/plain", msg)

	// Settings for SMTP server
	d := gomail.NewDialer("smtp.gmail.com", 587, e.from, e.pass)

	// This is only needed when SSL/TLS certificate is not valid on server.
	// In production this should be set to false.
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Now send E-Mail
	return d.DialAndSend(m)
}

func accessSecretVersion(version string) ([]byte, error) {

	// Create the client.
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create secretmanager client: %v", err)
	}
	defer client.Close()

	// Build the request.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: version,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to access secret version: %v", err)
	}

	log.Printf("retrieved payload for: %s\n", result.Name)
	return result.Payload.Data, nil
}

// clusterNotoficationPubSub consumes a CloudEvent message and extracts the
// Pub/Sub message.
func clusterNotoficationPubSub(ctx context.Context, e event.Event) error {
	var msg MessagePublishedData

	if err := e.DataAs(&msg); err != nil {
		return fmt.Errorf("event.DataAs: %w", err)
	}

	name := string(msg.Message.Data) // Automatically decoded from base64.

	log.Printf("Hello Nicklesh, %v, %v, %v, %v!", name, string(e.Data()), e.Source(), e.Context.String())
	return nil
}

func NClusterNotoficationPubSub(ctx context.Context, m PubSubMessage) error {
	data := string(m.Data) // Automatically decoded from base64.
	secret, err := accessSecretVersion("projects/109917119099/secrets/smtp_password/versions/latest")

	if err != nil {
		fmt.Errorf("Error accessing secret %v", err)
	}

	send_email := email{
		from: "nicklesh.adlakha@gmail.com",
		to:   "nicklesh.adlakha@ericsson.com",
		pass: string(secret),
	}

	log.Printf("Hello Nicklesh, %v, %s, %v\n", data, string(secret), err)

	return send_email.Send(data)
}
