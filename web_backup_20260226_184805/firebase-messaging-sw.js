// Firebase Messaging Service Worker for PWA background notifications

importScripts('https://www.gstatic.com/firebasejs/10.8.0/firebase-app-compat.js');
importScripts('https://www.gstatic.com/firebasejs/10.8.0/firebase-messaging-compat.js');
importScripts('firebase-config.js');

// Initialize Firebase in service worker
firebase.initializeApp(firebaseConfig);

// Retrieve an instance of Firebase Messaging
const messaging = firebase.messaging();

// Handle background messages
// Note: FCM automatically displays notifications when payload contains 'notification' field
// We only log here and handle data payload - no need to call showNotification manually
messaging.onBackgroundMessage((payload) => {
  console.log('Received background message:', payload);
  // FCM handles notification display automatically
  // Data is available in payload.data for custom handling
});

// Handle notification click
self.addEventListener('notificationclick', (event) => {
  console.log('Notification clicked:', event);
  event.notification.close();

  const data = event.notification.data || {};
  const urlToOpen = _buildUrlFromData(data);

  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      // Check if there's already a window open
      for (let i = 0; i < clientList.length; i++) {
        const client = clientList[i];
        if (client.url === urlToOpen && 'focus' in client) {
          return client.focus();
        }
      }
      // If no window is open, open a new one
      if (clients.openWindow) {
        return clients.openWindow(urlToOpen);
      }
    })
  );
});

// Build URL from notification data
function _buildUrlFromData(data) {
  const baseUrl = self.location.origin;

  if (!data || !data.objectType || !data.objectId) {
    return baseUrl;
  }

  const objectType = parseInt(data.objectType);
  const objectId = data.objectId;
  const ndcId = data.ndcId || '0';

  // ObjectType: 0 = user, 1 = blog, 2 = chat
  switch (objectType) {
    case 0:
      return `${baseUrl}/user/${objectId}?ndcId=${ndcId}`;
    case 1:
      return `${baseUrl}/blog/${objectId}?ndcId=${ndcId}`;
    case 2:
      return `${baseUrl}/chat/${objectId}?ndcId=${ndcId}`;
    default:
      return baseUrl;
  }
}
