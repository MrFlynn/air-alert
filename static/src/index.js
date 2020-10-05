import './index.scss'

// Get client current location.
if (!navigator.geolocation) {
  console.error("geolocation not supported");
} else {
  getPosition(getAQI)
}

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