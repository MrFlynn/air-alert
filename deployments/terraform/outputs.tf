output "nameservers" {
  value = [
    "ns1.digitalocean.com",
    "ns2.digitalocean.com",
    "ns3.digitalocean.com"
  ]

  description = "Remember to add these record to your registrar's NS entries."
}