package main

import (
	"log"
	"net/url"
	"strings"

	"github.com/multiplay/go-slack/chat"
	"github.com/multiplay/go-slack/webhook"
)

func init() { registerReporter(&reporterSlack{}) }

type reporterSlack struct {
	hook *webhook.Client
}

// InitializeFromURI retrieves the user input URI and must decide whether
// it can initialize from that or can't. If the URI is not suitable for the
// provider an errInitializationNotPossible error needs to be returned. If
// the initialization failed because of an error it must be returned.
func (r *reporterSlack) InitializeFromURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	if u.Scheme != "slack+https" {
		return errInitializationNotPossible
	}

	r.hook = webhook.New(strings.TrimPrefix(uri, "slack+"))
	return nil
}

// Execute takes the content of the reporting and executes the
// delivery of the message to the specified targets.
func (r reporterSlack) Execute(success bool, content, deploymentID, hostname string) error {
	// {
	//   "attachments": [
	//     {
	//       "color": "#36a64f",
	//       "fields": [
	//         {
	//           "short": true,
	//           "title": "Host",
	//           "value": "foobar"
	//         },
	//         {
	//           "short": true,
	//           "title": "Deployment-ID",
	//           "value": "12345"
	//         }
	//       ],
	//       "footer": "Deploy v0.1.0",
	//       "text": "And here’s an attachment!"
	//     }
	//   ],
	//   "text": "Deployment succeeded"
	// }

	log.Printf("%s", content)

	var (
		verb     = "failed"
		msgColor = "#a94442"
	)

	if success {
		verb = "succeeded"
		msgColor = "#3c763d"
	}

	payload := &chat.Message{}
	payload.Text = "Deployment " + verb

	payload.AddAttachment(&chat.Attachment{
		Color: msgColor,
		Text:  "```\n" + content + "```",
		Fields: []*chat.Field{
			{
				Title: "Host",
				Value: hostname,
				Short: true,
			},
			{
				Title: "Deployment-ID",
				Value: deploymentID,
				Short: true,
			},
			{
				Title: "Software Identifier",
				Value: cfg.SoftwareIdentifier,
				Short: true,
			},
		},
		Footer: "deploy " + version,
	})

	_, err := payload.Send(r.hook)
	return err
}
