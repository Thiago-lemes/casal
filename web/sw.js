self.addEventListener('push', (event) => {
  let data = { title: 'Casal', body: '', url: '/' };
  try { data = { ...data, ...event.data.json() }; } catch (_) {}
  // no iOS toda mensagem precisa mostrar uma notificação
  event.waitUntil(
    self.registration.showNotification(data.title, { body: data.body, data: { url: data.url } })
  );
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  event.waitUntil(clients.openWindow(event.notification.data.url || '/'));
});
