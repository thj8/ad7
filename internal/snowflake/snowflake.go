package snowflake

import (
	"crypto/rand"
	"fmt"
)

// Next generates a UUID v4 without hyphens (32 characters)
func Next() string {
	uuid := make([]byte, 16)
	_, _ = rand.Read(uuid)
	// Set version to 4 (random)
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant to RFC4122
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x%x%x%x%x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)
}
