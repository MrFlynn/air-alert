import './index.scss'

if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/worker.js');
}

// Get client current location.
if (!navigator.geolocation) {
  console.error("geolocation not supported");
} else {
  getPosition(getAQI)
}

// Register click handler to toggle modal open and closed.
document.querySelectorAll("#toggle-modal").forEach(e => {
  e.onclick = toggleModal;
});

// Register handler for subscribe button.
document.getElementById('subscribe-button').onclick = subscribe;
document.getElementById('stop-notifications').onclick = unsubscribe;

function getPosition(callback) {
  navigator.geolocation.getCurrentPosition(
    pos => callback(pos.coords.latitude, pos.coords.longitude),
    err => console.error(err)
  )
}

function getAQI(latitude, longitude) {
  fetch(`/aqi/current?lat=${latitude}&long=${longitude}`)
    .then(resp => resp.text())
    .then(aqi => updateDisplayBox(aqi))
    .catch(err => console.error(err));
}

function updateDisplayBox(aqi) {
  document.getElementById("aqi-value").textContent = aqi

  let aqiNum = parseFloat(aqi);
  let color = "hsl(141, 53%, 53%)";

  if (aqiNum > 50.0 && aqiNum <= 200) {
    color = "hsl(48, 100%, 67%)"
  } else if (aqiNum > 200.0) {
    color = "hsl(348, 100%, 61%)"
  }

  document.getElementById("aqi-box").style.backgroundColor = color
}

function toggleModal() {
  let target = document.getElementById('subscribe-modal')
  if (target.classList.contains('is-active')) {
    target.classList.remove('is-active');
  } else {
    target.classList.add('is-active');
    showCancelSubscriptionButton();
  }
}

function showCancelSubscriptionButton() {
  navigator.serviceWorker.ready.then(reg => {
    return reg.pushManager.getSubscription().then(sub => {
      if (sub !== null) {
        document.getElementById('stop-notifications').style.display = 'block';
      }
    });
  });
}

// Entrypoint to subscribe to notifications.
function subscribe() {
  navigator.serviceWorker.ready.then(reg => {
    return reg.pushManager.getSubscription().then(async sub => {
      if (sub) {
        return sub;
      }

      let resp = await fetch("/subscribe/key");
      let vapidPublicKey = await resp.arrayBuffer();

      return reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: vapidPublicKey
      });
    });
  }).then(sub => {
    sendSubscription(sub)
    toggleModal();
  });
}

function sendSubscription(subscription) {
  let threshold = parseInt(
    document.getElementById('threshold-preferences').value, 
    10
  );

  getPosition((lat, long) => {
    fetch('/subscribe', {
      method: 'post',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        subscription: subscription,
        latitude: lat,
        longitude: long,
        threshold: threshold
      })
    });
  });
}

function unsubscribe() {
  navigator.serviceWorker.ready.then(reg => {
    reg.pushManager.getSubscription().then(sub => {
      fetch('/unsubscribe', {
        method: 'delete',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          subscription: sub
        })
      }).then(_ => {
        sub.unsubscribe();
        toggleModal();
      });
    });
  });
}