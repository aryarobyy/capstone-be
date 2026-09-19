// Install firebase and bundle this module with your frontend.
import { initializeApp, getApps, getApp } from 'firebase/app';
import { getMessaging, getToken, onMessage, isSupported } from 'firebase/messaging';

// Call from the login button so browsers can show the permission prompt.
// Keep the returned access token in session memory. Never store the password.
export async function loginWithNotifications({ firebaseConfig, vapidKey, apiBase, email, password, onNotification = () => {} }) {
  let deviceToken;
  let messaging;
  let notificationError;
  try {
    if (await isSupported()) {
      if (await Notification.requestPermission() === 'granted') {
        const app = getApps().length ? getApp() : initializeApp(firebaseConfig);
        messaging = getMessaging(app);
        const registration = await navigator.serviceWorker.register('/firebase-messaging-sw.js');
        await navigator.serviceWorker.ready;
        deviceToken = await getToken(messaging, { vapidKey, serviceWorkerRegistration: registration });
      }
    }
  } catch (error) {
    // FCM setup errors must not prevent login; expose the error to the UI.
    notificationError = error.message;
  }
  const response = await fetch(`${apiBase}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password, ...(deviceToken ? { device_token: deviceToken, device_type: 'web' } : {}) }),
  });
  const body = await response.json();
  if (!response.ok) throw new Error(body.message || `Login failed: ${response.status}`);
  const session = body.data;
  const unsubscribe = deviceToken ? onMessage(messaging, onNotification) : () => {};
  return {
    session,
    notificationError,
    async logout() {
      try {
        const response = await fetch(`${apiBase}/api/auth/logout`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${session.access_token}` },
        });
        if (!response.ok && response.status !== 401) {
          throw new Error(`Server logout failed: ${response.status}; local session cleared but server revocation is unconfirmed`);
        }
      } finally {
        unsubscribe();
        session.access_token = '';
      }
    },
  };
}
