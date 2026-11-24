package schemaregistry

import "strings"

const IDSeparator = "___"

func formatSchemaVersionID(subject string) string {
	return subject
}

func extractSchemaVersionID(id string) string {
	return id
}

// errorDetail returns err's message, with a hint when Schema Registry
// rejected the request for missing SNI (Confluent Platform 8.0+, Jetty 12).
func errorDetail(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "Invalid SNI") {
		msg += "\n\nConfluent Platform 8.0+ Schema Registry requires TLS SNI on every request. " +
			"Go only sends SNI when schema_registry_url uses a hostname, not an IP address: " +
			"use the registry's DNS name, or set sni.host.check.enabled=false on the registry."
	}
	return msg
}
