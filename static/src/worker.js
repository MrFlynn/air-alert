self.addEventListener('push', e => {
  e.waitUntil(
    self.registration.showNotification('Air Alert', {
      body: e.data.text()
    })
  );
});