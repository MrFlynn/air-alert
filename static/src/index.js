import './index.scss'

if (!navigator.geolocation) {
  console.error("geolocation not supported")
} else {
  navigator.geolocation.getCurrentPosition(l => {
    getAQI(l.coords.latitude, l.coords.longitude)
  },
  e => console.error(e))
}

function getAQI(latitude, longitude) {
  fetch(`/aqi/current?lat=${latitude}&long=${longitude}`)
    .then(resp => resp.text())
    .then(data => document.getElementById("aqi").textContent = data)
    .catch(e => console.error(e))
}