package notification

import (
	"context"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
)

// NewMessaging uses Application Default Credentials (including GOOGLE_APPLICATION_CREDENTIALS).
// FCM does not require a Firestore database.
func NewMessaging(ctx context.Context, projectID string) (*messaging.Client, error) {
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	return app.Messaging(ctx)
}
