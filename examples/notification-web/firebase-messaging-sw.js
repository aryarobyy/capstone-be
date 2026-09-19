// Bundle this entry into /firebase-messaging-sw.js with your frontend build tool.
import { initializeApp } from 'firebase/app';
import { getMessaging, onBackgroundMessage } from 'firebase/messaging/sw';

const firebaseConfig = {
  apiKey: 'YOUR_PUBLIC_FIREBASE_WEB_API_KEY',
  projectId: 'YOUR_FIREBASE_PROJECT_ID',
  messagingSenderId: 'YOUR_MESSAGING_SENDER_ID',
  appId: 'YOUR_FIREBASE_WEB_APP_ID',
};
const messaging = getMessaging(initializeApp(firebaseConfig));
onBackgroundMessage(messaging, (payload) => {
  // FCM automatically displays notification payloads in the background.
  // Avoid showNotification() here: that would display the same push twice.
  // Forward to open pages for inbox synchronization if needed.
  self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clients) => {
    for (const client of clients) client.postMessage({ type: 'FCM_NOTIFICATION', payload });
  });
});
